package client

import (
	"context"
	"fmt"

	"github.com/Kpeewu/tissi-mah/pkg/grpcutil"
	filepb "github.com/Kpeewu/tissi-mah/services/user-service/proto/gen/filepb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// FileServiceClient est le client gRPC vers file-service.
// Les appels sont non-bloquants : retourne "" en cas d'erreur (dégradation gracieuse).
type FileServiceClient struct {
	conn       *grpc.ClientConn
	grpcClient filepb.FileServiceClient
	logger     *zap.Logger
}

// NewFileServiceClient établit la connexion gRPC vers file-service.
// address doit être au format "host:port" (ex: "file-service:50053").
func NewFileServiceClient(address string, logger *zap.Logger) (*FileServiceClient, error) {
	log := logger.Named("file-client")
	log.Debug("connexion au service file", zap.String("address", address))

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(grpcutil.ClientTransportCredentials()),
	)
	if err != nil {
		log.Error("échec de la connexion au service file", zap.Error(err), zap.String("address", address))
		return nil, fmt.Errorf("file-service: failed to connect to %s: %w", address, err)
	}

	return &FileServiceClient{
		conn:       conn,
		grpcClient: filepb.NewFileServiceClient(conn),
		logger:     log,
	}, nil
}

// Close libère la connexion gRPC. À appeler au shutdown du service.
func (c *FileServiceClient) Close() error {
	return c.conn.Close()
}

// GetCurrentDocumentURL retourne l'URL présignée fraîche du document courant
// du type donné, "" si non trouvé ou en cas d'erreur (dégradation gracieuse).
func (c *FileServiceClient) GetCurrentDocumentURL(ctx context.Context, userID, documentType string) string {
	resp, err := c.grpcClient.GetCurrentUserDocument(ctx, &filepb.GetCurrentUserDocumentRequest{
		UserId:       userID,
		DocumentType: documentType,
	})
	if err != nil {
		c.logger.Debug("document courant introuvable pour URL fraîche",
			zap.String("user_id", userID),
			zap.String("document_type", documentType),
			zap.Error(err),
		)
		return ""
	}
	return resp.DocumentUrl
}

// GetDocumentExpiry retourne la date d'expiration du document courant (expired_at),
// ou "" si aucun document n'est trouvé ou en cas d'erreur (dégradation gracieuse).
func (c *FileServiceClient) GetDocumentExpiry(ctx context.Context, userID, documentType string) string {
	resp, err := c.grpcClient.GetCurrentUserDocument(ctx, &filepb.GetCurrentUserDocumentRequest{
		UserId:       userID,
		DocumentType: documentType,
	})
	if err != nil {
		c.logger.Warn("impossible de récupérer l'expiration du document",
			zap.String("user_id", userID),
			zap.String("document_type", documentType),
			zap.Error(err),
		)
		return ""
	}
	return resp.ExpiredAt
}
