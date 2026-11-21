package integration

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	mongoHelper "github.com/Kpeewu/tissi-mah/pkg-test/mongodb"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/repository/implementations"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
)

var (
	testCollection *mongo.Collection
	testLogger     *zap.Logger
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	testLogger = zap.NewNop()

	testMongo, err := mongoHelper.SetupTestMongoDB(ctx)
	if err != nil {
		log.Fatalf("Failed to setup test MongoDB: %v", err)
	}

	testCollection = testMongo.Database().Collection("users")

	// Créer les index nécessaires
	if err := implementations.EnsureIndexes(ctx, testCollection); err != nil {
		log.Fatalf("Failed to ensure indexes: %v", err)
	}

	code := m.Run()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := testMongo.CleanUp(ctx); err != nil {
		log.Printf("Failed to cleanup test MongoDB: %v", err)
	}

	os.Exit(code)
}
