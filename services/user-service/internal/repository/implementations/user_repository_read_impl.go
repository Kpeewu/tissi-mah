package implementations

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/user-service/internal/repository/interfaces"
	userErrors "github.com/Kpeewu/tissi-mah/services/user-service/pkg/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
)

type userReadRepository struct {
	collection *mongo.Collection
	logger     *zap.Logger
}

func NewUserReadRepository(collection *mongo.Collection, logger *zap.Logger) repoInterfaces.UserRepositoryRead {
	return &userReadRepository{
		collection: collection,
		logger:     logger.Named("read-repo"),
	}
}

// GetByUserID récupère un utilisateur par son UserID
func (r *userReadRepository) GetByUserID(ctx context.Context, userID string) (*domain.User, error) {
	r.logger.Debug("recherche utilisateur par userID", zap.String("user_id", userID))
	return r.findOne(ctx, bson.M{"user_id": userID, "deleted_at": nil})
}

// GetByAuthID récupère un utilisateur par son AuthID
func (r *userReadRepository) GetByAuthID(ctx context.Context, authID string) (*domain.User, error) {
	r.logger.Debug("recherche utilisateur par authID", zap.String("auth_id", authID))
	return r.findOne(ctx, bson.M{"auth_id": authID, "deleted_at": nil})
}

// GetByFirebaseID récupère un utilisateur par son FirebaseID
func (r *userReadRepository) GetByFirebaseID(ctx context.Context, firebaseID string) (*domain.User, error) {
	r.logger.Debug("recherche utilisateur par firebaseID", zap.String("firebase_id", firebaseID))
	return r.findOne(ctx, bson.M{"firebase_id": firebaseID, "deleted_at": nil})
}

// findOne exécute une recherche unique avec mapping d'erreurs
func (r *userReadRepository) findOne(ctx context.Context, filter bson.M) (*domain.User, error) {
	r.logger.Debug("exécution findOne", zap.String("filter", fmt.Sprintf("%v", filter)))

	var user domain.User
	err := r.collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			r.logger.Debug("aucun document trouvé", zap.String("filter", fmt.Sprintf("%v", filter)))
			return nil, userErrors.ErrorUserNotFound
		}
		r.logger.Error("erreur lors de la recherche MongoDB", zap.Error(err), zap.String("filter", fmt.Sprintf("%v", filter)))
		return nil, userErrors.ErrorDataRetrievalFailed
	}
	return &user, nil
}
