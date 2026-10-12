package grpcutil

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"google.golang.org/grpc/metadata"
)

const (
	// MetadataUID est le nom de la metadata gRPC portant le Firebase UID validé par l'api-gateway.
	MetadataUID = "x-firebase-uid"

	// MetadataUIDSig est le nom de la metadata gRPC portant la signature HMAC
	// de x-firebase-uid, signée par l'api-gateway avec INTERNAL_HMAC_SECRET.
	MetadataUIDSig = "x-firebase-uid-sig"
)

// SignUID retourne le HMAC-SHA256 hex de uid signé avec secret.
// Appelé par l'api-gateway dans le metadataAnnotator gRPC pour garantir
// que la metadata x-firebase-uid vient bien de l'api-gateway et n'a pas
// été forgée par un pod interne.
func SignUID(secret []byte, uid string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(uid))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyUID vérifie que sig est la signature HMAC-SHA256 valide de uid.
// Appelé par les intercepteurs gRPC des services internes.
// Retourne false si la signature est invalide ou absente.
func VerifyUID(secret []byte, uid, sig string) bool {
	if uid == "" || sig == "" {
		return false
	}
	expected := SignUID(secret, uid)
	return hmac.Equal([]byte(sig), []byte(expected))
}

// ForwardIdentity recopie l'identité de l'appelant (x-firebase-uid et sa signature)
// de la metadata entrante vers la metadata sortante. À utiliser quand un service appelle
// un autre service pour le compte de l'utilisateur : le service appelé résout alors
// l'appelant depuis l'UID signé par l'api-gateway au lieu de faire confiance au body.
// Retourne ctx inchangé si aucune identité n'est présente.
func ForwardIdentity(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	uids := md.Get(MetadataUID)
	if len(uids) == 0 || uids[0] == "" {
		return ctx
	}
	ctx = metadata.AppendToOutgoingContext(ctx, MetadataUID, uids[0])
	if sigs := md.Get(MetadataUIDSig); len(sigs) > 0 && sigs[0] != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, MetadataUIDSig, sigs[0])
	}
	return ctx
}
