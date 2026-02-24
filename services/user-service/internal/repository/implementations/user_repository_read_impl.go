package implementations

import (
	"context"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/user-service/internal/repository/interfaces"
	userErrors "github.com/Kpeewu/tissi-mah/services/user-service/pkg/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type userReadRepository struct {
	collection *mongo.Collection
}

func NewUserReadRepository(collection *mongo.Collection) repoInterfaces.UserRepositoryRead {
	return &userReadRepository{collection: collection}
}

// GetByUserID récupère un utilisateur par son UserID
func (r *userReadRepository) GetByUserID(ctx context.Context, userID string) (*domain.User, error) {
	return r.findOne(ctx, bson.M{"user_id": userID, "deleted_at": nil})
}

// GetByAuthID récupère un utilisateur par son AuthID
func (r *userReadRepository) GetByAuthID(ctx context.Context, authID string) (*domain.User, error) {
	return r.findOne(ctx, bson.M{"auth_id": authID, "deleted_at": nil})
}

// GetByFirebaseID récupère un utilisateur par son FirebaseID
func (r *userReadRepository) GetByFirebaseID(ctx context.Context, firebaseID string) (*domain.User, error) {
	return r.findOne(ctx, bson.M{"firebase_id": firebaseID, "deleted_at": nil})
}

// findOne exécute une recherche unique avec mapping d'erreurs
func (r *userReadRepository) findOne(ctx context.Context, filter bson.M) (*domain.User, error) {
	var user domain.User
	err := r.collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, userErrors.ErrorUserNotFound
		}
		return nil, userErrors.ErrorDataRetrievalFailed
	}
	return &user, nil
}
