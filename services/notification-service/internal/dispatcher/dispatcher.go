package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/notification-service/internal/repository/interfaces"
)

// Event est la structure d'un événement de notification reçu via Redis Stream.
type Event struct {
	EventID       string            `json:"event_id"`
	EventType     string            `json:"event_type"`
	UserID        string            `json:"user_id"`
	UserIDs       []string          `json:"user_ids"`
	ReferenceID   string            `json:"reference_id"`
	ReferenceType string            `json:"reference_type"`
	Payload       map[string]string `json:"payload"`
}

// Dispatcher orchestre le traitement d'un événement de notification.
type Dispatcher struct {
	routingRepo      repoInterfaces.RoutingRepository
	templateRepo     repoInterfaces.TemplateRepository
	notificationRepo repoInterfaces.NotificationRepository
	inboxRepo        repoInterfaces.InboxRepository
	preferenceRepo   repoInterfaces.PreferenceRepository
	deviceRepo       repoInterfaces.DeviceTokenRepository
	pushClient       client.PushClient
	emailClient      client.EmailClient
	userClient       client.UserClient
	logger           *zap.Logger
}

func NewDispatcher(
	routingRepo repoInterfaces.RoutingRepository,
	templateRepo repoInterfaces.TemplateRepository,
	notificationRepo repoInterfaces.NotificationRepository,
	inboxRepo repoInterfaces.InboxRepository,
	preferenceRepo repoInterfaces.PreferenceRepository,
	deviceRepo repoInterfaces.DeviceTokenRepository,
	pushClient client.PushClient,
	emailClient client.EmailClient,
	userClient client.UserClient,
	logger *zap.Logger,
) *Dispatcher {
	return &Dispatcher{
		routingRepo:      routingRepo,
		templateRepo:     templateRepo,
		notificationRepo: notificationRepo,
		inboxRepo:        inboxRepo,
		preferenceRepo:   preferenceRepo,
		deviceRepo:       deviceRepo,
		pushClient:       pushClient,
		emailClient:      emailClient,
		userClient:       userClient,
		logger:           logger,
	}
}

// ProcessRaw parse le JSON brut d'un message Redis et dispatch l'événement.
func (d *Dispatcher) ProcessRaw(ctx context.Context, raw string) error {
	var event Event
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		d.logger.Error("failed to unmarshal event", zap.Error(err))
		return err
	}

	return d.Process(ctx, &event)
}

// Process traite un événement de notification.
// Pour un envoi groupé (user_ids non vide), itère sur chaque utilisateur.
func (d *Dispatcher) Process(ctx context.Context, event *Event) error {
	if len(event.UserIDs) > 0 {
		for _, uid := range event.UserIDs {
			singleEvent := &Event{
				EventID:       fmt.Sprintf("%s-%s", event.EventID, uid),
				EventType:     event.EventType,
				UserID:        uid,
				ReferenceID:   event.ReferenceID,
				ReferenceType: event.ReferenceType,
				Payload:       event.Payload,
			}
			if err := d.processForUser(ctx, singleEvent); err != nil {
				d.logger.Error("failed to process for user",
					zap.String("user_id", uid),
					zap.String("event_type", event.EventType),
					zap.Error(err),
				)
			}
		}
		return nil
	}

	return d.processForUser(ctx, event)
}

// processForUser traite un événement pour un seul utilisateur.
func (d *Dispatcher) processForUser(ctx context.Context, event *Event) error {
	d.logger.Debug("processing event",
		zap.String("event_id", event.EventID),
		zap.String("event_type", event.EventType),
		zap.String("user_id", event.UserID),
	)

	// 1. Récupérer le routing
	routing, err := d.routingRepo.GetByEventType(ctx, event.EventType)
	if err != nil {
		d.logger.Error("routing not found, skipping event", zap.String("event_type", event.EventType))
		return nil
	}

	// 2. Récupérer les préférences utilisateur
	prefs, err := d.preferenceRepo.GetByUserID(ctx, event.UserID)
	if err != nil {
		d.logger.Error("failed to get preferences", zap.String("user_id", event.UserID), zap.Error(err))
		return err
	}

	// 3. Récupérer les infos utilisateur
	userInfo, err := d.userClient.GetUserByUserID(ctx, event.UserID)
	if err != nil {
		d.logger.Error("failed to get user info", zap.String("user_id", event.UserID), zap.Error(err))
		return err
	}

	lang := userInfo.LanguageCode
	if lang == "" {
		lang = "fr"
	}

	maxAttempts := domain.MaxAttemptsForPriority(routing.Priority)

	// 4. Push
	if routing.SendPush && prefs.PushEnabled {
		d.dispatchPush(ctx, event, routing, userInfo, lang, maxAttempts)
	}

	// 5. Email — fallback sur le payload si le user-service ne retourne pas l'email
	emailAddr := userInfo.Email
	if emailAddr == "" {
		emailAddr = event.Payload["email"]
	}
	if routing.SendEmail && prefs.EmailEnabled && emailAddr != "" {
		userInfo.Email = emailAddr
		d.dispatchEmail(ctx, event, routing, userInfo, lang, maxAttempts)
	}

	// 6. Inbox
	if routing.CreateInboxEntry {
		d.dispatchInbox(ctx, event, lang)
	}

	return nil
}

// dispatchPush envoie la notification push à tous les tokens actifs de l'utilisateur.
func (d *Dispatcher) dispatchPush(ctx context.Context, event *Event, routing *domain.EventRouting, userInfo *client.UserInfo, lang string, maxAttempts int16) {
	// Déduplication
	exists, _ := d.notificationRepo.ExistsByEventIDAndChannel(ctx, event.EventID, "push")
	if exists {
		d.logger.Debug("push already sent, skipping", zap.String("event_id", event.EventID))
		return
	}

	// Template
	tmpl, err := d.templateRepo.GetByEventTypeAndChannel(ctx, event.EventType, "push", lang)
	if err != nil {
		d.logger.Error("push template not found", zap.String("event_type", event.EventType))
		return
	}

	resolved := domain.ResolveTemplate(tmpl, event.Payload)

	// Tokens actifs
	tokens, err := d.deviceRepo.GetActiveByUserID(ctx, event.UserID)
	if err != nil || len(tokens) == 0 {
		d.logger.Debug("no active device tokens", zap.String("user_id", event.UserID))
		return
	}

	// Construire la liste de FCM tokens
	fcmTokens := make([]string, len(tokens))
	for i, t := range tokens {
		fcmTokens[i] = t.FCMToken
	}

	// Enregistrer le log notification
	notif := &domain.Notification{
		EventID:      event.EventID,
		UserID:       event.UserID,
		EventType:    event.EventType,
		Channel:      "push",
		TemplateID:   tmpl.TemplateID,
		ResolvedTitle: resolved.Title,
		ResolvedBody: resolved.Body,
		Status:       "processing",
		ReferenceID:  event.ReferenceID,
		ReferenceType: event.ReferenceType,
		AttemptCount: 1,
		MaxAttempts:  maxAttempts,
		ProviderName: "fcm",
	}
	if err := d.notificationRepo.Create(ctx, notif); err != nil {
		d.logger.Error("failed to log push notification", zap.Error(err))
		return
	}

	// Envoi via push-service
	data := event.Payload
	if data == nil {
		data = make(map[string]string)
	}
	data["event_type"] = event.EventType

	if len(fcmTokens) == 1 {
		success, errCode, err := d.pushClient.SendPush(ctx, resolved.Title, resolved.Body, fcmTokens[0], data)
		if err != nil || !success {
			d.notificationRepo.UpdateStatus(ctx, notif.NotificationID, "failed", errCode, "") //nolint:errcheck
			return
		}
	} else {
		_, _, err := d.pushClient.SendPushMulticast(ctx, resolved.Title, resolved.Body, fcmTokens, data)
		if err != nil {
			d.notificationRepo.UpdateStatus(ctx, notif.NotificationID, "failed", err.Error(), "") //nolint:errcheck
			return
		}
	}

	d.notificationRepo.UpdateStatus(ctx, notif.NotificationID, "sent", "", "") //nolint:errcheck
}

// dispatchEmail envoie la notification par email.
func (d *Dispatcher) dispatchEmail(ctx context.Context, event *Event, routing *domain.EventRouting, userInfo *client.UserInfo, lang string, maxAttempts int16) {
	// Déduplication
	exists, _ := d.notificationRepo.ExistsByEventIDAndChannel(ctx, event.EventID, "email")
	if exists {
		d.logger.Debug("email already sent, skipping", zap.String("event_id", event.EventID))
		return
	}

	// Template
	tmpl, err := d.templateRepo.GetByEventTypeAndChannel(ctx, event.EventType, "email", lang)
	if err != nil {
		d.logger.Error("email template not found", zap.String("event_type", event.EventType))
		return
	}

	resolved := domain.ResolveTemplate(tmpl, event.Payload)

	// Enregistrer le log notification
	notif := &domain.Notification{
		EventID:          event.EventID,
		UserID:           event.UserID,
		EventType:        event.EventType,
		Channel:          "email",
		TemplateID:       tmpl.TemplateID,
		ResolvedTitle:    resolved.Title,
		ResolvedSubject:  resolved.Subject,
		ResolvedBody:     resolved.Body,
		RecipientAddress: userInfo.Email,
		Status:           "processing",
		ReferenceID:      event.ReferenceID,
		ReferenceType:    event.ReferenceType,
		AttemptCount:     1,
		MaxAttempts:      maxAttempts,
		ProviderName:     "email",
	}
	if err := d.notificationRepo.Create(ctx, notif); err != nil {
		d.logger.Error("failed to log email notification", zap.Error(err))
		return
	}

	// Envoi via email-service
	success, errCode, err := d.emailClient.SendEmail(ctx, userInfo.Email, resolved.Subject, resolved.Body, resolved.BodyHTML)
	if err != nil || !success {
		failReason := errCode
		if err != nil {
			failReason = err.Error()
		}
		d.notificationRepo.UpdateStatus(ctx, notif.NotificationID, "failed", failReason, "") //nolint:errcheck
		return
	}

	d.notificationRepo.UpdateStatus(ctx, notif.NotificationID, "sent", "", "") //nolint:errcheck
}

// dispatchInbox crée une entrée dans l'inbox de l'utilisateur.
func (d *Dispatcher) dispatchInbox(ctx context.Context, event *Event, lang string) {
	tmpl, err := d.templateRepo.GetByEventTypeAndChannel(ctx, event.EventType, "push", lang)
	if err != nil {
		d.logger.Error("inbox template not found", zap.String("event_type", event.EventType))
		return
	}

	resolved := domain.ResolveTemplate(tmpl, event.Payload)

	entry := &domain.InboxEntry{
		UserID:     event.UserID,
		EventType:  event.EventType,
		Title:      resolved.Title,
		Body:       resolved.Body,
		ActionType: event.ReferenceType,
		ActionID:   event.ReferenceID,
	}

	if err := d.inboxRepo.Create(ctx, entry); err != nil {
		d.logger.Error("failed to create inbox entry", zap.Error(err))
	}
}
