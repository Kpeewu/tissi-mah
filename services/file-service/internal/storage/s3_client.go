package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	fileconfig "github.com/Kpeewu/tissi-mah/services/file-service/internal/config"
)

type s3Client struct {
	client   *s3.Client
	bucket   string
	endpoint string
}

// NewS3Client crée un client S3 compatible MinIO/AWS.
// Si S3_ENDPOINT est défini, le client pointe vers MinIO (dev).
// Sinon, il utilise AWS S3 standard (prod).
func NewS3Client(ctx context.Context, cfg fileconfig.S3Config) (StorageClient, error) {
	var opts []func(*awsconfig.LoadOptions) error

	opts = append(opts, awsconfig.WithRegion(cfg.Region))
	opts = append(opts, awsconfig.WithCredentialsProvider(
		credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	))

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	var s3Opts []func(*s3.Options)

	// Si un endpoint custom est défini (MinIO), on l'utilise
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = cfg.ForcePathStyle
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	return &s3Client{
		client:   client,
		bucket:   cfg.Bucket,
		endpoint: cfg.Endpoint,
	}, nil
}

// Upload envoie un fichier vers S3/MinIO
func (s *s3Client) Upload(ctx context.Context, key string, data io.Reader, contentType string, size int64) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          data,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return s.GenerateURL(key), nil
}

// Delete supprime un fichier de S3/MinIO
func (s *s3Client) Delete(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

// GenerateURL retourne l'URL publique d'un fichier
func (s *s3Client) GenerateURL(key string) string {
	if s.endpoint != "" {
		// MinIO : http://localhost:9000/bucket/key
		return fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, key)
	}
	// AWS S3 : https://bucket.s3.region.amazonaws.com/key
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, key)
}
