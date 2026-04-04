package dispatcher_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/dispatcher"
	"github.com/Kpeewu/tissi-mah/services/notification-service/internal/domain"
	notifErrors "github.com/Kpeewu/tissi-mah/services/notification-service/pkg/errors"
	"github.com/Kpeewu/tissi-mah/services/notification-service/tests/mocks"
)

// newDispatcher crée un dispatcher avec tous les dépendances mockées.
func newDispatcher() (
	*dispatcher.Dispatcher,
	*mocks.MockRoutingRepository,
	*mocks.MockTemplateRepository,
	*mocks.MockNotificationRepository,
	*mocks.MockInboxRepository,
	*mocks.MockPreferenceRepository,
	*mocks.MockDeviceTokenRepository,
	*mocks.MockPushClient,
	*mocks.MockEmailClient,
	*mocks.MockUserClient,
) {
	routingRepo := new(mocks.MockRoutingRepository)
	templateRepo := new(mocks.MockTemplateRepository)
	notifRepo := new(mocks.MockNotificationRepository)
	inboxRepo := new(mocks.MockInboxRepository)
	prefRepo := new(mocks.MockPreferenceRepository)
	deviceRepo := new(mocks.MockDeviceTokenRepository)
	pushClient := new(mocks.MockPushClient)
	emailClient := new(mocks.MockEmailClient)
	userClient := new(mocks.MockUserClient)

	d := dispatcher.NewDispatcher(
		routingRepo, templateRepo, notifRepo, inboxRepo,
		prefRepo, deviceRepo, pushClient, emailClient, userClient,
		zap.NewNop(),
	)
	return d, routingRepo, templateRepo, notifRepo, inboxRepo, prefRepo, deviceRepo, pushClient, emailClient, userClient
}

// testRouting retourne un routing standard avec push + email + inbox activés.
func testRouting(eventType string) *domain.EventRouting {
	return &domain.EventRouting{
		RoutingID:        "routing-001",
		EventType:        eventType,
		SendPush:         true,
		SendEmail:        true,
		CreateInboxEntry: true,
		Priority:         "standard",
		IsActive:         true,
	}
}

// testTemplate retourne un template de test.
func testTemplate(eventType, channel string) *domain.Template {
	return &domain.Template{
		TemplateID:   "tmpl-" + channel,
		EventType:    eventType,
		Channel:      channel,
		LanguageCode: "fr",
		Title:        "Titre {{name}}",
		Subject:      "Sujet {{name}}",
		Body:         "Corps {{info}}",
	}
}

// testPrefs retourne des préférences avec tout activé.
func testPrefs() *domain.UserNotificationPreference {
	return &domain.UserNotificationPreference{
		UserID:       "user-123",
		PushEnabled:  true,
		EmailEnabled: true,
	}
}

// testUserInfo retourne un UserInfo de test avec email.
func testUserInfo() *client.UserInfo {
	return &client.UserInfo{
		UserID:       "user-123",
		Email:        "user@example.com",
		LanguageCode: "fr",
	}
}

// testEvent retourne un événement de test.
func testEvent(eventType string) *dispatcher.Event {
	return &dispatcher.Event{
		EventID:   "evt-test-001",
		EventType: eventType,
		UserID:    "user-123",
		Payload:   map[string]string{"name": "Kofi", "info": "détail"},
	}
}

// =============================================================================
// Process — routing introuvable
// =============================================================================

func TestProcess_RoutingNotFound_EventIgnored(t *testing.T) {
	d, routingRepo, _, _, _, _, _, pushClient, emailClient, _ := newDispatcher()
	ctx := context.Background()

	routingRepo.On("GetByEventType", mock.Anything, "UNKNOWN_EVENT").
		Return(nil, notifErrors.ErrorRoutingNotFound)

	err := d.Process(ctx, &dispatcher.Event{
		EventID:   "evt-no-routing",
		EventType: "UNKNOWN_EVENT",
		UserID:    "user-999",
	})

	// Pas d'erreur — l'événement est silencieusement ignoré
	assert.NoError(t, err)
	pushClient.AssertNotCalled(t, "SendPush")
	emailClient.AssertNotCalled(t, "SendEmail")
}

// =============================================================================
// Process — push désactivé dans les préférences
// =============================================================================

func TestProcess_PushDisabledByPreferences(t *testing.T) {
	d, routingRepo, templateRepo, notifRepo, inboxRepo, prefRepo, deviceRepo, pushClient, emailClient, userClient := newDispatcher()
	ctx := context.Background()
	event := testEvent("BOOKING_CONFIRMED")

	routing := testRouting("BOOKING_CONFIRMED")
	prefs := &domain.UserNotificationPreference{UserID: "user-123", PushEnabled: false, EmailEnabled: true}
	userInfo := testUserInfo()
	emailTmpl := testTemplate("BOOKING_CONFIRMED", "email")
	inboxTmpl := testTemplate("BOOKING_CONFIRMED", "push")

	routingRepo.On("GetByEventType", mock.Anything, "BOOKING_CONFIRMED").Return(routing, nil)
	prefRepo.On("GetByUserID", mock.Anything, "user-123").Return(prefs, nil)
	userClient.On("GetUserByUserID", mock.Anything, "user-123").Return(userInfo, nil)

	// Email : déduplication, template, envoi
	notifRepo.On("ExistsByEventIDAndChannel", mock.Anything, event.EventID, "email").Return(false, nil)
	templateRepo.On("GetByEventTypeAndChannel", mock.Anything, "BOOKING_CONFIRMED", "email", "fr").Return(emailTmpl, nil)
	notifRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	emailClient.On("SendEmail", mock.Anything, "user@example.com", mock.Anything, mock.Anything, mock.Anything).Return(true, "", nil)
	notifRepo.On("UpdateStatus", mock.Anything, mock.Anything, "sent", "", "").Return(nil)

	// Inbox
	templateRepo.On("GetByEventTypeAndChannel", mock.Anything, "BOOKING_CONFIRMED", "push", "fr").Return(inboxTmpl, nil)
	inboxRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	_ = deviceRepo // non appelé pour push

	err := d.Process(ctx, event)
	assert.NoError(t, err)
	pushClient.AssertNotCalled(t, "SendPush")
	pushClient.AssertNotCalled(t, "SendPushMulticast")
}

// =============================================================================
// Process — email désactivé dans les préférences
// =============================================================================

func TestProcess_EmailDisabledByPreferences(t *testing.T) {
	d, routingRepo, templateRepo, notifRepo, inboxRepo, prefRepo, deviceRepo, pushClient, emailClient, userClient := newDispatcher()
	ctx := context.Background()
	event := testEvent("BOOKING_CONFIRMED")

	routing := testRouting("BOOKING_CONFIRMED")
	prefs := &domain.UserNotificationPreference{UserID: "user-123", PushEnabled: true, EmailEnabled: false}
	userInfo := testUserInfo()
	pushTmpl := testTemplate("BOOKING_CONFIRMED", "push")
	inboxTmpl := testTemplate("BOOKING_CONFIRMED", "push")
	tokens := []*domain.UserDeviceToken{{TokenID: "tok-1", FCMToken: "fcm-abc", IsActive: true}}

	routingRepo.On("GetByEventType", mock.Anything, "BOOKING_CONFIRMED").Return(routing, nil)
	prefRepo.On("GetByUserID", mock.Anything, "user-123").Return(prefs, nil)
	userClient.On("GetUserByUserID", mock.Anything, "user-123").Return(userInfo, nil)

	// Push
	notifRepo.On("ExistsByEventIDAndChannel", mock.Anything, event.EventID, "push").Return(false, nil)
	templateRepo.On("GetByEventTypeAndChannel", mock.Anything, "BOOKING_CONFIRMED", "push", "fr").Return(pushTmpl, nil)
	deviceRepo.On("GetActiveByUserID", mock.Anything, "user-123").Return(tokens, nil)
	notifRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	pushClient.On("SendPush", mock.Anything, mock.Anything, mock.Anything, "fcm-abc", mock.Anything).Return(true, "", nil)
	notifRepo.On("UpdateStatus", mock.Anything, mock.Anything, "sent", "", "").Return(nil)

	// Inbox
	templateRepo.On("GetByEventTypeAndChannel", mock.Anything, "BOOKING_CONFIRMED", "push", "fr").Return(inboxTmpl, nil)
	inboxRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	err := d.Process(ctx, event)
	assert.NoError(t, err)
	emailClient.AssertNotCalled(t, "SendEmail")
}

// =============================================================================
// Process — email vide → SendEmail non appelé
// =============================================================================

func TestProcess_EmailEmptySkipsSendEmail(t *testing.T) {
	d, routingRepo, templateRepo, notifRepo, inboxRepo, prefRepo, deviceRepo, pushClient, emailClient, userClient := newDispatcher()
	ctx := context.Background()
	event := testEvent("BOOKING_CONFIRMED")

	routing := testRouting("BOOKING_CONFIRMED")
	prefs := testPrefs()
	userInfo := &client.UserInfo{UserID: "user-123", Email: "", LanguageCode: "fr"} // email vide
	pushTmpl := testTemplate("BOOKING_CONFIRMED", "push")
	tokens := []*domain.UserDeviceToken{{TokenID: "tok-1", FCMToken: "fcm-abc", IsActive: true}}

	routingRepo.On("GetByEventType", mock.Anything, "BOOKING_CONFIRMED").Return(routing, nil)
	prefRepo.On("GetByUserID", mock.Anything, "user-123").Return(prefs, nil)
	userClient.On("GetUserByUserID", mock.Anything, "user-123").Return(userInfo, nil)

	notifRepo.On("ExistsByEventIDAndChannel", mock.Anything, event.EventID, "push").Return(false, nil)
	templateRepo.On("GetByEventTypeAndChannel", mock.Anything, "BOOKING_CONFIRMED", "push", "fr").Return(pushTmpl, nil)
	deviceRepo.On("GetActiveByUserID", mock.Anything, "user-123").Return(tokens, nil)
	notifRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	pushClient.On("SendPush", mock.Anything, mock.Anything, mock.Anything, "fcm-abc", mock.Anything).Return(true, "", nil)
	notifRepo.On("UpdateStatus", mock.Anything, mock.Anything, "sent", "", "").Return(nil)

	// Inbox utilise le template push
	inboxRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	err := d.Process(ctx, event)
	assert.NoError(t, err)
	emailClient.AssertNotCalled(t, "SendEmail")
}

// =============================================================================
// Process — déduplication : push déjà envoyé
// =============================================================================

func TestProcess_DeduplicationPush(t *testing.T) {
	d, routingRepo, _, notifRepo, inboxRepo, prefRepo, deviceRepo, pushClient, _, userClient := newDispatcher()
	ctx := context.Background()
	event := testEvent("BOOKING_CONFIRMED")

	routing := &domain.EventRouting{EventType: "BOOKING_CONFIRMED", SendPush: true, SendEmail: false, CreateInboxEntry: false, Priority: "standard"}
	prefs := testPrefs()
	userInfo := testUserInfo()

	routingRepo.On("GetByEventType", mock.Anything, "BOOKING_CONFIRMED").Return(routing, nil)
	prefRepo.On("GetByUserID", mock.Anything, "user-123").Return(prefs, nil)
	userClient.On("GetUserByUserID", mock.Anything, "user-123").Return(userInfo, nil)
	// Déduplication : déjà existant
	notifRepo.On("ExistsByEventIDAndChannel", mock.Anything, event.EventID, "push").Return(true, nil)

	_ = deviceRepo
	_ = inboxRepo

	err := d.Process(ctx, event)
	assert.NoError(t, err)
	pushClient.AssertNotCalled(t, "SendPush")
}

// =============================================================================
// Process — push succès → UpdateStatus("sent")
// =============================================================================

func TestProcess_PushSuccess_StatusSent(t *testing.T) {
	d, routingRepo, templateRepo, notifRepo, _, prefRepo, deviceRepo, pushClient, _, userClient := newDispatcher()
	ctx := context.Background()
	event := testEvent("TRIP_STARTED")

	routing := &domain.EventRouting{
		EventType: "TRIP_STARTED", SendPush: true, SendEmail: false, CreateInboxEntry: false, Priority: "standard",
	}
	prefs := testPrefs()
	userInfo := testUserInfo()
	pushTmpl := testTemplate("TRIP_STARTED", "push")
	tokens := []*domain.UserDeviceToken{{TokenID: "tok-1", FCMToken: "fcm-token-ok", IsActive: true}}

	routingRepo.On("GetByEventType", mock.Anything, "TRIP_STARTED").Return(routing, nil)
	prefRepo.On("GetByUserID", mock.Anything, "user-123").Return(prefs, nil)
	userClient.On("GetUserByUserID", mock.Anything, "user-123").Return(userInfo, nil)
	notifRepo.On("ExistsByEventIDAndChannel", mock.Anything, event.EventID, "push").Return(false, nil)
	templateRepo.On("GetByEventTypeAndChannel", mock.Anything, "TRIP_STARTED", "push", "fr").Return(pushTmpl, nil)
	deviceRepo.On("GetActiveByUserID", mock.Anything, "user-123").Return(tokens, nil)
	notifRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	pushClient.On("SendPush", mock.Anything, mock.Anything, mock.Anything, "fcm-token-ok", mock.Anything).Return(true, "", nil)
	notifRepo.On("UpdateStatus", mock.Anything, mock.Anything, "sent", "", "").Return(nil)

	err := d.Process(ctx, event)
	assert.NoError(t, err)
	notifRepo.AssertCalled(t, "UpdateStatus", mock.Anything, mock.Anything, "sent", "", "")
}

// =============================================================================
// Process — push échec → UpdateStatus("failed")
// =============================================================================

func TestProcess_PushFailed_StatusFailed(t *testing.T) {
	d, routingRepo, templateRepo, notifRepo, _, prefRepo, deviceRepo, pushClient, _, userClient := newDispatcher()
	ctx := context.Background()
	event := testEvent("TRIP_STARTED")

	routing := &domain.EventRouting{
		EventType: "TRIP_STARTED", SendPush: true, SendEmail: false, CreateInboxEntry: false, Priority: "standard",
	}
	prefs := testPrefs()
	userInfo := testUserInfo()
	pushTmpl := testTemplate("TRIP_STARTED", "push")
	tokens := []*domain.UserDeviceToken{{TokenID: "tok-1", FCMToken: "fcm-bad", IsActive: true}}

	routingRepo.On("GetByEventType", mock.Anything, "TRIP_STARTED").Return(routing, nil)
	prefRepo.On("GetByUserID", mock.Anything, "user-123").Return(prefs, nil)
	userClient.On("GetUserByUserID", mock.Anything, "user-123").Return(userInfo, nil)
	notifRepo.On("ExistsByEventIDAndChannel", mock.Anything, event.EventID, "push").Return(false, nil)
	templateRepo.On("GetByEventTypeAndChannel", mock.Anything, "TRIP_STARTED", "push", "fr").Return(pushTmpl, nil)
	deviceRepo.On("GetActiveByUserID", mock.Anything, "user-123").Return(tokens, nil)
	notifRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	pushClient.On("SendPush", mock.Anything, mock.Anything, mock.Anything, "fcm-bad", mock.Anything).Return(false, "UNREGISTERED", nil)
	notifRepo.On("UpdateStatus", mock.Anything, mock.Anything, "failed", "UNREGISTERED", "").Return(nil)

	err := d.Process(ctx, event)
	assert.NoError(t, err)
	notifRepo.AssertCalled(t, "UpdateStatus", mock.Anything, mock.Anything, "failed", "UNREGISTERED", "")
}

// =============================================================================
// Process — create_inbox_entry = true → inboxRepo.Create appelé
// =============================================================================

func TestProcess_InboxEntryCreated(t *testing.T) {
	d, routingRepo, templateRepo, _, inboxRepo, prefRepo, _, pushClient, _, userClient := newDispatcher()
	ctx := context.Background()
	event := testEvent("BOOKING_CONFIRMED")

	routing := &domain.EventRouting{
		EventType: "BOOKING_CONFIRMED", SendPush: false, SendEmail: false, CreateInboxEntry: true, Priority: "standard",
	}
	prefs := testPrefs()
	userInfo := testUserInfo()
	inboxTmpl := testTemplate("BOOKING_CONFIRMED", "push")

	routingRepo.On("GetByEventType", mock.Anything, "BOOKING_CONFIRMED").Return(routing, nil)
	prefRepo.On("GetByUserID", mock.Anything, "user-123").Return(prefs, nil)
	userClient.On("GetUserByUserID", mock.Anything, "user-123").Return(userInfo, nil)
	templateRepo.On("GetByEventTypeAndChannel", mock.Anything, "BOOKING_CONFIRMED", "push", "fr").Return(inboxTmpl, nil)
	inboxRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	err := d.Process(ctx, event)
	assert.NoError(t, err)
	inboxRepo.AssertCalled(t, "Create", mock.Anything, mock.Anything)
	pushClient.AssertNotCalled(t, "SendPush")
}

// =============================================================================
// Process — envoi groupé (user_ids) → processForUser appelé pour chaque user
// =============================================================================

func TestProcess_GroupSend_PerUser(t *testing.T) {
	d, routingRepo, _, notifRepo, _, prefRepo, _, _, _, userClient := newDispatcher()
	ctx := context.Background()

	// Routing sans push/email/inbox pour simplifier
	routing := &domain.EventRouting{
		EventType: "SYSTEM_NOTICE", SendPush: false, SendEmail: false, CreateInboxEntry: false, Priority: "low",
	}
	prefs := &domain.UserNotificationPreference{PushEnabled: true, EmailEnabled: true}
	user1 := &client.UserInfo{UserID: "user-1", Email: "", LanguageCode: "fr"}
	user2 := &client.UserInfo{UserID: "user-2", Email: "", LanguageCode: "fr"}

	routingRepo.On("GetByEventType", mock.Anything, "SYSTEM_NOTICE").Return(routing, nil).Times(2)
	prefRepo.On("GetByUserID", mock.Anything, mock.Anything).Return(prefs, nil).Times(2)
	userClient.On("GetUserByUserID", mock.Anything, "user-1").Return(user1, nil)
	userClient.On("GetUserByUserID", mock.Anything, "user-2").Return(user2, nil)
	_ = notifRepo

	groupEvent := &dispatcher.Event{
		EventID:   "evt-group-001",
		EventType: "SYSTEM_NOTICE",
		UserIDs:   []string{"user-1", "user-2"},
		Payload:   map[string]string{},
	}

	err := d.Process(ctx, groupEvent)
	assert.NoError(t, err)
	userClient.AssertCalled(t, "GetUserByUserID", mock.Anything, "user-1")
	userClient.AssertCalled(t, "GetUserByUserID", mock.Anything, "user-2")
}

// =============================================================================
// ProcessRaw — erreur JSON
// =============================================================================

func TestProcessRaw_InvalidJSON(t *testing.T) {
	d, _, _, _, _, _, _, _, _, _ := newDispatcher()
	ctx := context.Background()

	err := d.ProcessRaw(ctx, "not-valid-json{{{")
	assert.Error(t, err)
}

// =============================================================================
// Process — userClient error → erreur propagée
// =============================================================================

func TestProcess_UserClientError(t *testing.T) {
	d, routingRepo, _, _, _, prefRepo, _, _, _, userClient := newDispatcher()
	ctx := context.Background()
	event := testEvent("BOOKING_CONFIRMED")

	routing := testRouting("BOOKING_CONFIRMED")
	prefs := testPrefs()

	routingRepo.On("GetByEventType", mock.Anything, "BOOKING_CONFIRMED").Return(routing, nil)
	prefRepo.On("GetByUserID", mock.Anything, "user-123").Return(prefs, nil)
	userClient.On("GetUserByUserID", mock.Anything, "user-123").Return(nil, errors.New("user-service unavailable"))

	err := d.Process(ctx, event)
	assert.Error(t, err)
}
