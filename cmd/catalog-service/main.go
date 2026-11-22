package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"lavka/internal/config"
	"lavka/internal/grpc/catalogservice"
	"lavka/internal/grpc/interceptors"
	"lavka/internal/logger"
	"lavka/internal/storage/ads"
	"lavka/internal/storage/favorites"
	"lavka/internal/storage/postgres"

	"google.golang.org/grpc"
)

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	cfg := config.MustLoadConfig()
	log := logger.SetupLogger(cfg.Env)

	grpcPort := envOrDefault("GRPC_PORT", "50052")
	metricsPort := envOrDefault("METRICS_PORT", "9092")

	var (
		adsStore catalogservice.AdsStore
		favStore catalogservice.FavoritesStore
		pgDB     *sql.DB
	)

	if cfg.DatabaseURL != "" {
		db, err := postgres.Open(context.Background(), cfg.DatabaseURL)
		if err != nil {
			log.Error("catalog-service: failed to connect to postgresql", "err", err)
			panic(err)
		}
		pgDB = db
		adsStore = postgres.NewAdsRepository(db, log)
		favStore = postgres.NewFavoritesRepository(db, log)
		log.Info("catalog-service: connected to PostgreSQL")
	} else {
		var adsPath string
		if cfg.StoragePath != "" {
			adsPath = filepath.Join(filepath.Dir(cfg.StoragePath), "ads.json")
		}
		adsRepo, err := ads.NewAdsRepositoryWithFile(adsPath)
		if err != nil {
			log.Error("catalog-service: failed to initialize ads repo", "err", err)
			panic(err)
		}
		adsStore = adsRepo
		favStore = favorites.NewInMemoryRepository()
		log.Info("catalog-service: using local JSON/in-memory storage")
	}

	metricsReg := interceptors.NewGRPCMetricsRegistry("catalog-service")
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(metricsReg.UnaryServerInterceptor(log)),
	)
	catalogservice.RegisterCatalogServiceServer(grpcServer, catalogservice.NewServer(adsStore, favStore))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Error("catalog-service: failed to listen", "port", grpcPort, "err", err)
		panic(err)
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metricsReg)
	metricsMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"catalog-service"}`))
	})
	metricsSrv := &http.Server{
		Addr:              fmt.Sprintf(":%s", metricsPort),
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("catalog-service metrics server started", slog.String("addr", metricsSrv.Addr))
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("catalog-service metrics server error", "err", err)
		}
	}()

	go func() {
		log.Info("catalog-service gRPC server started", slog.String("addr", lis.Addr().String()))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("catalog-service gRPC server error", "err", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	sig := <-stop

	log.Info("stopping catalog-service", slog.String("signal", sig.String()))
	grpcServer.GracefulStop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = metricsSrv.Shutdown(ctx)
	if pgDB != nil {
		_ = pgDB.Close()
	}
	log.Info("catalog-service stopped")
}
