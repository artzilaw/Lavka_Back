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
	"syscall"
	"time"

	"lavka/internal/config"
	"lavka/internal/grpc/authservice"
	"lavka/internal/grpc/interceptors"
	"lavka/internal/logger"
	"lavka/internal/services/auth"
	"lavka/internal/storage"
	"lavka/internal/storage/blacklist"
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

	grpcPort := envOrDefault("GRPC_PORT", "50051")
	metricsPort := envOrDefault("METRICS_PORT", "9091")

	var (
		userStore auth.UserProviderSaver
		pgDB      *sql.DB
	)

	if cfg.DatabaseURL != "" {
		db, err := postgres.Open(context.Background(), cfg.DatabaseURL)
		if err != nil {
			log.Error("auth-service: failed to connect to postgresql", "err", err)
			panic(err)
		}
		pgDB = db
		userStore = postgres.NewUserRepository(db, log)
		log.Info("auth-service: connected to PostgreSQL")
	} else {
		st, err := storage.New(cfg.StoragePath)
		if err != nil {
			log.Error("auth-service: failed to initialize storage", "err", err)
			panic(err)
		}
		userStore = st
		log.Info("auth-service: using local JSON/in-memory storage")
	}

	tokenBlacklist := blacklist.New(cfg.CleanupInterval)
	authSvc := auth.New(log, userStore, tokenBlacklist, cfg.TokenTTL, cfg.TokenSecret)

	metricsReg := interceptors.NewGRPCMetricsRegistry("auth-service")
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(metricsReg.UnaryServerInterceptor(log)),
		grpc.MaxRecvMsgSize(10<<20), // 10 MB для передачи аватарок
	)
	authservice.RegisterAuthServiceServer(grpcServer, authservice.NewServer(authSvc, tokenBlacklist))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Error("auth-service: failed to listen", "port", grpcPort, "err", err)
		panic(err)
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metricsReg)
	metricsMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"auth-service"}`))
	})
	metricsSrv := &http.Server{
		Addr:              fmt.Sprintf(":%s", metricsPort),
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("auth-service metrics server started", slog.String("addr", metricsSrv.Addr))
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("auth-service metrics server error", "err", err)
		}
	}()

	go func() {
		log.Info("auth-service gRPC server started", slog.String("addr", lis.Addr().String()))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("auth-service gRPC server error", "err", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	sig := <-stop

	log.Info("stopping auth-service", slog.String("signal", sig.String()))
	grpcServer.GracefulStop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = metricsSrv.Shutdown(ctx)
	tokenBlacklist.Stop()
	if pgDB != nil {
		_ = pgDB.Close()
	}
	log.Info("auth-service stopped")
}
