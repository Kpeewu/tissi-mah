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

// UploadProfilePicture uploade imageBytes vers file-service via UploadUserDocument (streaming).
// Retourne l'URL S3 du document créé, ou une erreur (appel bloquant).
func (c *FileServiceClient) UploadProfilePicture(ctx context.Context, userID string, imageBytes []byte) (string, error) {
	stream, err := c.grpcClient.UploadUserDocument(ctx)
	if err != nil {
		c.logger.Error("impossible d'ouvrir le stream d'upload", zap.String("user_id", userID), zap.Error(err))
		return "", fmt.Errorf("file-service: open stream: %w", err)
	}

	err = stream.Send(&filepb.UploadUserDocumentRequest{
		Data: &filepb.UploadUserDocumentRequest_Metadata{
			Metadata: &filepb.UserDocumentMetadata{
				UserId:        userID,
				DocumentName:  "profile_picture",
				DocumentType:  "profilePicture",
				MimeType:      "image/jpeg",
				FileSizeBytes: int64(len(imageBytes)),
			},
		},
	})
	if err != nil {
		c.logger.Error("échec envoi métadonnées profile picture", zap.String("user_id", userID), zap.Error(err))
		return "", fmt.Errorf("file-service: send metadata: %w", err)
	}

	err = stream.Send(&filepb.UploadUserDocumentRequest{
		Data: &filepb.UploadUserDocumentRequest_Chunk{
			Chunk: imageBytes,
		},
	})
	if err != nil {
		c.logger.Error("échec envoi chunk profile picture", zap.String("user_id", userID), zap.Error(err))
		return "", fmt.Errorf("file-service: send chunk: %w", err)
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		c.logger.Error("échec réception réponse upload", zap.String("user_id", userID), zap.Error(err))
		return "", fmt.Errorf("file-service: close stream: %w", err)
	}

	c.logger.Debug("profile picture uploadée", zap.String("user_id", userID), zap.String("url", resp.DocumentUrl))
	return resp.DocumentUrl, nil
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
