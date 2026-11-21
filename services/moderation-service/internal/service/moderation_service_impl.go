package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/client"
	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/filter"
	"github.com/Kpeewu/tissi-mah/services/moderation-service/internal/provider"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/moderation-service/internal/repository/interfaces"
	serviceInterfaces "github.com/Kpeewu/tissi-mah/services/moderation-service/internal/service/interfaces"
	"go.uber.org/zap"
)

type moderationServiceImpl struct {
	logRepo        repoInterfaces.ModerationLogRepository
	violationRepo  repoInterfaces.ViolationRepository
	textFilter     *filter.TextFilterImpl
	imageFilter    func(data []byte, mimeType string) filter.ImageFilterResult
	textProvider   provider.TextModerator
	imageProvider  provider.ImageModerator
	userClient     client.UserClient
	authClient     client.AuthClient
	blockedThresh  float32
	flaggedThresh  float32
	failClosed     bool
	logger         *zap.Logger
}

type Config struct {
	LogRepo        repoInterfaces.ModerationLogRepository
	ViolationRepo  repoInterfaces.ViolationRepository
	TextProvider   provider.TextModerator  // nil = pas d'API externe
	ImageProvider  provider.ImageModerator // nil = pas d'API externe
	UserClient     client.UserClient
	AuthClient     client.AuthClient
	BlockedThresh  float32
	FlaggedThresh  float32
	FailClosed     bool
	Logger         *zap.Logger
}

func NewModerationService(cfg Config) serviceInterfaces.ModerationService {
	return &moderationServiceImpl{
		logRepo:       cfg.LogRepo,
		violationRepo: cfg.ViolationRepo,
		textFilter:    filter.NewTextFilter(),
		imageFilter:   filter.ValidateImage,
		textProvider:  cfg.TextProvider,
		imageProvider: cfg.ImageProvider,
		userClient:    cfg.UserClient,
		authClient:    cfg.AuthClient,
		blockedThresh: cfg.BlockedThresh,
		flaggedThresh: cfg.FlaggedThresh,
		failClosed:    cfg.FailClosed,
		logger:        cfg.Logger,
	}
}

// ModerateText analyse un message texte selon le flux hybride :
// filtre local d'abord, puis Perspective API si ambigu.
func (s *moderationServiceImpl) ModerateText(ctx context.Context, contentID, contentType, text, authorID string) (*domain.ModerationResult, error) {
	result := &domain.ModerationResult{}

	// 1. Filtre local
	localResult := s.textFilter.Analyze(text)
	if localResult.Score >= s.blockedThresh {
		result.Decision = domain.DecisionBlocked
		result.Category = domain.CategoryObscene
		result.Score = localResult.Score
		result.Reason = fmt.Sprintf("local filter matched: %s", localResult.Matched)
		s.saveLog(ctx, contentID, contentType, authorID, result)
		return result, nil
	}

	// 2. API externe si ambigu ou si score intermédiaire
	if s.textProvider != nil && localResult.Score >= s.flaggedThresh/2 {
		apiScore, apiCategory, err := s.textProvider.ModerateText(ctx, text)
		if err == nil && apiScore > 0 {
			result.UsedFallback = true
			if apiScore > localResult.Score {
				localResult.Score = apiScore
				result.Category = apiCategory
			}
		}
	}

	result.Score = localResult.Score
	if result.Category == "" {
		result.Category = domain.CategoryNone
	}

	switch {
	case result.Score >= s.blockedThresh:
		result.Decision = domain.DecisionBlocked
		result.Reason = "content blocked by moderation"
	case result.Score >= s.flaggedThresh:
		result.Decision = domain.DecisionFlagged
		result.Reason = "content flagged for review"
	default:
		result.Decision = domain.DecisionApproved
		result.Category = domain.CategoryNone
	}

	s.saveLog(ctx, contentID, contentType, authorID, result)
	return result, nil
}

// ModerateImage analyse une image selon le flux hybride :
// validation locale puis Google Vision SafeSearch.
// En cas de BLOCKED, déclenche la suspension progressive du compte.
func (s *moderationServiceImpl) ModerateImage(ctx context.Context, contentID, contentType string, imageBytes []byte, mimeType, authorID string) (*domain.ModerationResult, error) {
	result := &domain.ModerationResult{}

	// 1. Validation locale (format, taille, magic bytes)
	localResult := filter.ValidateImage(imageBytes, mimeType)
	if !localResult.Valid {
		result.Decision = domain.DecisionBlocked
		result.Category = domain.CategoryNone
		result.Score = 1.0
		result.Reason = localResult.Reason
		s.saveLog(ctx, contentID, contentType, authorID, result)
		return result, nil
	}

	// 2. API externe (Google Vision)
	if s.imageProvider != nil {
		apiScore, apiCategory, err := s.imageProvider.ModerateImage(ctx, imageBytes, mimeType)
		if err == nil && apiScore > 0 {
			result.UsedFallback = true
			result.Score = apiScore
			result.Category = apiCategory
		}
	} else if s.failClosed {
		// fail-closed sans API = BLOCKED
		result.Decision = domain.DecisionBlocked
		result.Score = 1.0
		result.Reason = "image moderation API unavailable (fail-closed)"
		s.saveLog(ctx, contentID, contentType, authorID, result)
		return result, nil
	}

	switch {
	case result.Score >= s.blockedThresh:
		result.Decision = domain.DecisionBlocked
		result.Reason = fmt.Sprintf("image blocked: %s detected", result.Category)
	case result.Score >= s.flaggedThresh:
		result.Decision = domain.DecisionFlagged
		result.Reason = "image flagged for review"
	default:
		result.Decision = domain.DecisionApproved
		result.Category = domain.CategoryNone
	}

	s.saveLog(ctx, contentID, contentType, authorID, result)

	// 3. Blocage progressif si BLOCKED (photo obscène)
	if result.Decision == domain.DecisionBlocked && authorID != "" {
		s.applySuspension(ctx, authorID, contentType)
	}

	return result, nil
}

// applySuspension détermine la durée de suspension progressive et appelle auth-service.
func (s *moderationServiceImpl) applySuspension(ctx context.Context, userID, contentType string) {
	count, err := s.violationRepo.CountByUserAndType(ctx, userID, contentType)
	if err != nil {
		s.logger.Error("failed to count violations for suspension", zap.Error(err), zap.String("userID", userID))
		return
	}
	nextCount := count + 1

	suspendedUntil, isBanned := domain.SuspensionDuration(nextCount)

	// Enregistrer la violation
	v := &domain.UserViolation{
		UserID:          userID,
		ContentType:     contentType,
		ViolationCount:  nextCount,
		SuspensionUntil: suspendedUntil,
		IsPermanentBan:  isBanned,
		CreatedAt:       time.Now().UTC(),
	}
	if err := s.violationRepo.Create(ctx, v); err != nil {
		s.logger.Error("failed to save violation", zap.Error(err), zap.String("userID", userID))
	}

	// Résoudre user_id → auth_id pour appeler auth-service
	if s.userClient == nil || s.authClient == nil {
		s.logger.Warn("user/auth client not configured, skipping account suspension")
		return
	}

	authID, err := s.userClient.GetAuthIDByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to resolve auth_id for suspension", zap.Error(err), zap.String("userID", userID))
		return
	}

	if err := s.authClient.SuspendAccount(ctx, authID, suspendedUntil, isBanned); err != nil {
		s.logger.Error("failed to suspend account", zap.Error(err),
			zap.String("authID", authID),
			zap.Bool("isBanned", isBanned),
		)
	} else {
		s.logger.Info("account suspended",
			zap.String("userID", userID),
			zap.String("authID", authID),
			zap.Int("violationCount", nextCount),
			zap.Bool("isBanned", isBanned),
		)
	}
}

func (s *moderationServiceImpl) saveLog(ctx context.Context, contentID, contentType, authorID string, result *domain.ModerationResult) {
	log := &domain.ModerationLog{
		ContentID:    contentID,
		ContentType:  contentType,
		AuthorID:     authorID,
		Decision:     result.Decision,
		Category:     result.Category,
		Score:        result.Score,
		Reason:       result.Reason,
		UsedFallback: result.UsedFallback,
	}
	if err := s.logRepo.Create(ctx, log); err != nil {
		s.logger.Error("failed to save moderation log", zap.Error(err))
	}
}
