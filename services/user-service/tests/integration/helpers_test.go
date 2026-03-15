package integration

import (
	"context"
	"testing"

	"github.com/Kpeewu/tissi-mah/services/user-service/internal/domain"
	"github.com/Kpeewu/tissi-mah/services/user-service/internal/repository/implementations"
	repoInterfaces "github.com/Kpeewu/tissi-mah/services/user-service/internal/repository/interfaces"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// newReadRepo crée une instance du repository de lecture pour les tests.
func newReadRepo() repoInterfaces.UserRepositoryRead {
	return implementations.NewUserReadRepository(testCollection, testLogger)
}

// newWriteRepo crée une instance du repository d'écriture pour les tests.
func newWriteRepo() repoInterfaces.UserRepositoryWrite {
	return implementations.NewUserWriteRepository(testCollection, testLogger)
}

// insertUser insère directement un utilisateur dans la collection de test.
func insertUser(t *testing.T, user *domain.User) {
	t.Helper()
	_, err := testCollection.InsertOne(context.Background(), user)
	require.NoError(t, err)
}

// cleanCollection vide la collection avant chaque test pour l'isolation.
func cleanCollection(t *testing.T) {
	t.Helper()
	_, err := testCollection.DeleteMany(context.Background(), bson.M{})
	require.NoError(t, err)
}
