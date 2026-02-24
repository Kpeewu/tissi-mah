package implementations

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnsureIndexes crée les index nécessaires sur la collection users.
// À appeler au démarrage du service après connexion MongoDB.
func EnsureIndexes(ctx context.Context, collection *mongo.Collection) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetName("idx_user_id"),
		},
		{
			Keys: bson.D{{Key: "auth_id", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetName("idx_auth_id").
				SetPartialFilterExpression(bson.M{"deleted_at": nil}),
		},
		{
			Keys: bson.D{{Key: "firebase_id", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetName("idx_firebase_id").
				SetPartialFilterExpression(bson.M{"deleted_at": nil}),
		},
		{
			Keys: bson.D{{Key: "deleted_at", Value: 1}},
			Options: options.Index().
				SetSparse(true).
				SetName("idx_deleted_at"),
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}
