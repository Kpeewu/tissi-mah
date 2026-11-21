// Package grpcutil provides shared gRPC utilities for all services.
package grpcutil

import (
	"crypto/tls"

	"google.golang.org/grpc/credentials"
)

// ClientTransportCredentials retourne les credentials TLS à utiliser pour
// tout dial gRPC intra-cluster. Le certificat serveur n'est pas vérifié :
// la confiance s'appuie sur le trust boundary du cluster Kubernetes.
func ClientTransportCredentials() credentials.TransportCredentials {
	return credentials.NewTLS(&tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // intra-cluster; MITM hors modèle de menace
		MinVersion:         tls.VersionTLS12,
	})
}
