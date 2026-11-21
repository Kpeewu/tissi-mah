package mongodb

import (
	"context"
	"fmt"

	mongodbModule "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/mongo"
	mongoOptions "go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	defaultMongoImage = "mongo:6"
	defaultDatabase   = "test_db"
)

// TestMongoDB encapsule le container MongoDB de test et le client.
type TestMongoDB struct {
	Client    *mongo.Client
	Container *mongodbModule.MongoDBContainer
	URI       string
}

// CleanUp arrête le container et ferme le client.
func (tm *TestMongoDB) CleanUp(ctx context.Context) error {
	if tm.Client != nil {
		if err := tm.Client.Disconnect(ctx); err != nil {
			return fmt.Errorf("failed to disconnect mongo client: %w", err)
		}
	}
	if tm.Container != nil {
		tm.Container.Terminate(ctx)
	}
	return nil
}

// Database retourne la base de données de test.
func (tm *TestMongoDB) Database() *mongo.Database {
	return tm.Client.Database(defaultDatabase)
}

// SetupTestMongoDB démarre un container MongoDB et retourne un client connecté.
func SetupTestMongoDB(ctx context.Context) (*TestMongoDB, error) {
	container, err := mongodbModule.Run(ctx, defaultMongoImage)
	if err != nil {
		return nil, fmt.Errorf("failed to start mongodb container: %w", err)
	}

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	client, err := mongo.Connect(mongoOptions.Client().ApplyURI(uri))
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(ctx)
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	return &TestMongoDB{
		Client:    client,
		Container: container,
		URI:       uri,
	}, nil
}
