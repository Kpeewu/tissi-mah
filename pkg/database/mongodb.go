// Package database provides shared database utilities for all services.
package database

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// MongoConfig holds MongoDB connection configuration.
type MongoConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// NewMongoClient creates a new MongoDB client from a config struct.
func NewMongoClient(ctx context.Context, cfg MongoConfig) (*mongo.Client, error) {
	uri := fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database,
	)
	return NewMongoClientFromURL(ctx, uri)
}

// NewMongoClientFromURL creates a MongoDB client from a connection URL.
func NewMongoClientFromURL(ctx context.Context, mongoURL string) (*mongo.Client, error) {
	opts := options.Client().ApplyURI(mongoURL)

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	return client, nil
}
