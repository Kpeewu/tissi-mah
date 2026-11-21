package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/grpc/metadata"
)

// cleanCollection vide la collection users entre chaque test.
func cleanCollection(t *testing.T) {
	t.Helper()
	_, err := testCollection.DeleteMany(context.Background(), bson.M{})
	require.NoError(t, err)
}

// ctxWithUID retourne un contexte gRPC avec le Firebase UID injecté en metadata.
func ctxWithUID(uid string) context.Context {
	md := metadata.Pairs("x-firebase-uid", uid)
	return metadata.NewOutgoingContext(context.Background(), md)
}
