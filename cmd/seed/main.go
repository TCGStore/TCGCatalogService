package main

import (
	"context"
	_ "embed"
	"log/slog"
	"os"

	"github.com/Javiercatalanv/ArquitecturaSistemas/services/catalog-service/internal/catalog"
	"github.com/Javiercatalanv/ArquitecturaSistemas/services/catalog-service/internal/config"
)

// Product ids here are shared with inventory-service's seed; keep them in sync.
//
//go:embed products.json
var productsJSON []byte

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}

	client, db, err := config.ConnectMongo(ctx, cfg)
	if err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
	defer client.Disconnect(ctx)

	count, err := catalog.NewRepository(db).Seed(ctx, productsJSON)
	if err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
	slog.Info("seed complete", "products", count, "db", cfg.MongoDB)
}
