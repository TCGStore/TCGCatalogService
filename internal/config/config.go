package config

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type Config struct {
	GRPCPort string
	MongoURI string
	MongoDB  string
}

func Load() (Config, error) {
	cfg := Config{
		GRPCPort: getEnv("GRPC_PORT", "50051"),
		MongoURI: os.Getenv("MONGO_URI"),
		MongoDB:  getEnv("MONGO_DB", "catalog"),
	}
	if cfg.MongoURI == "" {
		return cfg, fmt.Errorf("MONGO_URI is required, e.g. mongodb://user:pass@localhost:27017")
	}
	return cfg, nil
}

// ConnectMongo fails fast if MongoDB isn't reachable, instead of on the first query.
func ConnectMongo(ctx context.Context, cfg Config) (*mongo.Client, *mongo.Database, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return nil, nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, nil, fmt.Errorf("ping mongodb: %w", err)
	}
	return client, client.Database(cfg.MongoDB), nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
