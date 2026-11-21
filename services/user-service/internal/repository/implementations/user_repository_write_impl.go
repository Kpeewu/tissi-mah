package implementations

import (
	"context"
	"time"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/user-service/internal/repository/interfaces"
	userErrors "github.com/Kpeewu/tissi-mah/services/user-service/pkg/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
)

type userWriteRepository struct {
	collection *mongo.Collection
	logger     *zap.Logger
}

func NewUserWriteRepository(collection *mongo.Collection, logger *zap.Logger) repoInterfaces.UserRepositoryWrite {
	return &userWriteRepository{
		collection: collection,
		logger:     logger.Named("write-repo"),
	}
}

// Create insère un nouveau document utilisateur dans MongoDB
func (r *userWriteRepository) Create(ctx context.Context, user *domain.User) (string, error) {
	r.logger.Debug("création utilisateur", zap.String("user_id", user.UserID), zap.String("auth_id", user.AuthID))

	_, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return "", userErrors.ErrorProfileAlreadyExists
		}
		r.logger.Error("erreur lors de l'insertion MongoDB", zap.Error(err), zap.String("user_id", user.UserID))
		return "", userErrors.ErrorInternalServer
	}

	r.logger.Info("utilisateur créé avec succès", zap.String("user_id", user.UserID))
	return user.UserID, nil
}

// Update met à jour un document utilisateur existant
func (r *userWriteRepository) Update(ctx context.Context, user *domain.User) (*domain.User, error) {
	r.logger.Debug("mise à jour utilisateur", zap.String("user_id", user.UserID))

	user.UpdatedAt = time.Now().UTC()

	filter := bson.M{"user_id": user.UserID, "deleted_at": nil}
	update := bson.M{"$set": user}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updated domain.User
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updated)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, userErrors.ErrorUserNotFound
		}
		r.logger.Error("erreur lors de la mise à jour MongoDB", zap.Error(err), zap.String("user_id", user.UserID))
		return nil, userErrors.ErrorInternalServer
	}

	r.logger.Info("utilisateur mis à jour avec succès", zap.String("user_id", user.UserID))
	return &updated, nil
}

// AnonymizeAndDelete anonymise les données personnelles et effectue un soft-delete
func (r *userWriteRepository) AnonymizeAndDelete(ctx context.Context, user *domain.User) error {
	r.logger.Debug("anonymisation et suppression utilisateur", zap.String("user_id", user.UserID))

	filter := bson.M{"user_id": user.UserID, "deleted_at": nil}
	update := bson.M{"$set": user}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		r.logger.Error("erreur lors de l'anonymisation MongoDB", zap.Error(err), zap.String("user_id", user.UserID))
		return userErrors.ErrorInternalServer
	}
	if result.MatchedCount == 0 {
		return userErrors.ErrorUserNotFound
	}

	r.logger.Info("utilisateur anonymisé et supprimé avec succès", zap.String("user_id", user.UserID))
	return nil
}
