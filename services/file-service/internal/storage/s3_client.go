package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	fileconfig "github.com/Kpeewu/tissi-mah/services/file-service/internal/config"
	"go.uber.org/zap"
)

type s3Client struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	endpoint      string
	logger        *zap.Logger
}

// NewS3Client crée un client S3 compatible MinIO/AWS.
// Si S3_ENDPOINT est défini, le client pointe vers MinIO (dev).
// Sinon, il utilise AWS S3 standard (prod).
func NewS3Client(ctx context.Context, cfg fileconfig.S3Config, logger *zap.Logger) (StorageClient, error) {
	var opts []func(*awsconfig.LoadOptions) error

	opts = append(opts, awsconfig.WithRegion(cfg.Region))
	opts = append(opts, awsconfig.WithCredentialsProvider(
		credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	))

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		logger.Error("failed to load AWS config", zap.Error(err))
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	var s3Opts []func(*s3.Options)

	// Si un endpoint custom est défini (MinIO), on l'utilise
	if cfg.Endpoint != "" {
		ep := cfg.Endpoint
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(ep)
			o.UsePathStyle = cfg.ForcePathStyle
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	// Client dédié aux URL présignées : si S3_PUBLIC_ENDPOINT est défini, les URL
	// générées utilisent ce domaine (HTTPS public) plutôt que l'endpoint interne.
	presignEndpoint := cfg.PublicEndpoint
	if presignEndpoint == "" {
		presignEndpoint = cfg.Endpoint
	}
	var presignOpts []func(*s3.Options)
	if presignEndpoint != "" {
		ep := presignEndpoint
		presignOpts = append(presignOpts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(ep)
			o.UsePathStyle = cfg.ForcePathStyle
		})
	}
	presignBaseClient := s3.NewFromConfig(awsCfg, presignOpts...)

	return &s3Client{
		client:        client,
		presignClient: s3.NewPresignClient(presignBaseClient),
		bucket:        cfg.Bucket,
		endpoint:      cfg.Endpoint,
		logger:        logger,
	}, nil
}

// Upload envoie un fichier vers S3/MinIO
func (s *s3Client) Upload(ctx context.Context, key string, data io.Reader, contentType string, size int64) (string, error) {
	s.logger.Debug("uploading to S3",
		zap.String("key", key),
		zap.String("contentType", contentType),
		zap.Int64("size", size),
	)

	// Convertir en reader seekable pour le calcul de checksum AWS SDK v2 (sans TLS)
	bodyBytes, err := io.ReadAll(data)
	if err != nil {
		s.logger.Error("failed to read upload data", zap.Error(err), zap.String("key", key))
		return "", fmt.Errorf("failed to read upload data: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(bodyBytes),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}

	_, err = s.client.PutObject(ctx, input)
	if err != nil {
		s.logger.Error("S3 upload failed", zap.Error(err), zap.String("key", key))
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	url := s.generateURL(key)
	s.logger.Info("S3 upload success", zap.String("key", key), zap.String("url", url))
	return url, nil
}

// Delete supprime un fichier de S3/MinIO
func (s *s3Client) Delete(ctx context.Context, key string) error {
	s.logger.Debug("deleting from S3", zap.String("key", key))

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObject(ctx, input)
	if err != nil {
		s.logger.Error("S3 delete failed", zap.Error(err), zap.String("key", key))
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	s.logger.Debug("S3 delete success", zap.String("key", key))
	return nil
}

// generateURL retourne l'URL publique d'un fichier (usage interne uniquement).
func (s *s3Client) generateURL(key string) string {
	if s.endpoint != "" {
		return fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, key)
	}
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, key)
}

// GeneratePresignedURL génère une URL signée à durée de vie limitée (bucket privé).
func (s *s3Client) GeneratePresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		s.logger.Error("presign URL failed", zap.String("key", key), zap.Error(err))
		return "", fmt.Errorf("presign failed: %w", err)
	}
	return req.URL, nil
}
