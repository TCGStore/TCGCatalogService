package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/Javiercatalanv/ArquitecturaSistemas/services/catalog-service/gen/catalogpb"
	"github.com/Javiercatalanv/ArquitecturaSistemas/services/catalog-service/internal/catalog"
	"github.com/Javiercatalanv/ArquitecturaSistemas/services/catalog-service/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("catalog-service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	mongoClient, db, err := config.ConnectMongo(ctx, cfg)
	if err != nil {
		return err
	}
	defer mongoClient.Disconnect(context.Background())

	listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	catalogpb.RegisterCatalogServiceServer(grpcServer, catalog.NewServer(catalog.NewRepository(db)))

	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	// Lets grpcurl and Postman discover the API without the .proto file.
	reflection.Register(grpcServer)

	go func() {
		<-ctx.Done()
		slog.Info("shutting down")
		healthServer.Shutdown()
		grpcServer.GracefulStop()
	}()

	slog.Info("catalog-service listening", "port", cfg.GRPCPort, "db", cfg.MongoDB)
	return grpcServer.Serve(listener)
}
