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
)

type userWriteRepository struct {
	collection *mongo.Collection
}

func NewUserWriteRepository(collection *mongo.Collection) repoInterfaces.UserRepositoryWrite {
	return &userWriteRepository{collection: collection}
}

// Create insère un nouveau document utilisateur dans MongoDB
func (r *userWriteRepository) Create(ctx context.Context, user *domain.User) (string, error) {
	_, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return "", userErrors.ErrorProfileAlreadyExists
		}
		return "", userErrors.ErrorInternalServer
	}
	return user.UserID, nil
}

// Update met à jour un document utilisateur existant
func (r *userWriteRepository) Update(ctx context.Context, user *domain.User) (*domain.User, error) {
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
		return nil, userErrors.ErrorInternalServer
	}

	return &updated, nil
}

// Delete effectue un soft-delete sur le document utilisateur
func (r *userWriteRepository) Delete(ctx context.Context, userID string) error {
	now := time.Now().UTC()
	filter := bson.M{"user_id": userID, "deleted_at": nil}
	update := bson.M{"$set": bson.M{
		"deleted_at": now,
		"updated_at": now,
	}}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return userErrors.ErrorInternalServer
	}
	if result.MatchedCount == 0 {
		return userErrors.ErrorUserNotFound
	}

	return nil
}
