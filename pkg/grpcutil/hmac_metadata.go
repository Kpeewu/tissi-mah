package grpcutil

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

const (
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
