package grpcutil

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestForwardIdentity(t *testing.T) {
	t.Run("recopie l'UID et sa signature", func(t *testing.T) {
		in := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
			MetadataUID, "uid-123",
			MetadataUIDSig, "sig-abc",
			"authorization", "Bearer secret",
		))

		out, ok := metadata.FromOutgoingContext(ForwardIdentity(in))
		if !ok {
			t.Fatal("metadata sortante absente")
		}
		if got := out.Get(MetadataUID); len(got) != 1 || got[0] != "uid-123" {
			t.Errorf("x-firebase-uid = %v, attendu [uid-123]", got)
		}
		if got := out.Get(MetadataUIDSig); len(got) != 1 || got[0] != "sig-abc" {
			t.Errorf("x-firebase-uid-sig = %v, attendu [sig-abc]", got)
		}
		if got := out.Get("authorization"); len(got) != 0 {
			t.Errorf("seule l'identité doit être propagée, authorization = %v", got)
		}
	})

	t.Run("sans identite entrante : contexte inchange", func(t *testing.T) {
		ctx := context.Background()
		if _, ok := metadata.FromOutgoingContext(ForwardIdentity(ctx)); ok {
			t.Error("aucune metadata sortante attendue")
		}
	})
}
