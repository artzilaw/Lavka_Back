package app

import (
	"context"
	"database/sql"
	"log/slog"
	"path/filepath"

	httpapp "lavka/internal/app/http"
	"lavka/internal/app/http/middleware"
	"lavka/internal/config"
	"lavka/internal/grpc/authservice"
	"lavka/internal/grpc/catalogservice"
	"lavka/internal/services/auth"
	"lavka/internal/storage"
	"lavka/internal/storage/ads"
	"lavka/internal/storage/blacklist"
	"lavka/internal/storage/cart"
	"lavka/internal/storage/favorites"
	"lavka/internal/storage/postgres"
	"lavka/internal/storage/wallet"
)

type App struct {
	HTTPServer    *httpapp.App
	Blacklist     *blacklist.InMemory
	DB            *sql.DB
	authClient    *authservice.Client
	catalogClient *catalogservice.Client
}

func New(log *slog.Logger, cfg *config.Config) *App {
	var (
		userStore    auth.UserProviderSaver
		adsService   httpapp.Ads
		cartService  httpapp.Cart
		favService   httpapp.Favorites
		walletSvc    httpapp.Wallet
		authService  httpapp.Auth
		tokenChecker middleware.TokenChecker
		localBL      *blacklist.InMemory
		pgDB         *sql.DB
		authCl       *authservice.Client
		catalogCl    *catalogservice.Client
	)

	if cfg.DatabaseURL != "" {
		db, err := postgres.Open(context.Background(), cfg.DatabaseURL)
		if err != nil {
			log.Error("failed to connect to postgresql", "err", err)
			panic(err)
		}
		pgDB = db
		userStore = postgres.NewUserRepository(db, log)
		adsService = postgres.NewAdsRepository(db, log)
		cartService = postgres.NewCartRepository(db, log)
		favService = postgres.NewFavoritesRepository(db, log)
		walletSvc = postgres.NewWalletRepository(db, log)
		log.Info("connected to PostgreSQL storage")
	} else {
		st, err := storage.New(cfg.StoragePath)
		if err != nil {
			log.Error("failed to initialize storage", "err", err)
			panic(err)
		}

		var adsPath, cartPath string
		if cfg.StoragePath != "" {
			dir := filepath.Dir(cfg.StoragePath)
			adsPath = filepath.Join(dir, "ads.json")
			cartPath = filepath.Join(dir, "cart.json")
		}

		adsRepo, err := ads.NewAdsRepositoryWithFile(adsPath)
		if err != nil {
			log.Error("failed to initialize ads repository", "err", err)
			panic(err)
		}

		cartRepo, err := cart.NewCartRepository(cartPath)
		if err != nil {
			log.Error("failed to initialize cart repository", "err", err)
			panic(err)
		}

		userStore = st
		adsService = adsRepo
		cartService = cartRepo
		favService = favorites.NewInMemoryRepository()
		walletSvc = wallet.NewInMemoryRepository()
		log.Info("using local JSON/in-memory storage (DATABASE_URL not set)")
	}

	if cfg.AuthGRPCAddr != "" {
		cl, err := authservice.NewClient(cfg.AuthGRPCAddr)
		if err != nil {
			log.Error("failed to connect to auth gRPC service", "addr", cfg.AuthGRPCAddr, "err", err)
			panic(err)
		}
		authCl = cl
		authService = cl
		tokenChecker = cl
		log.Info("connected to auth-service via gRPC", slog.String("addr", cfg.AuthGRPCAddr))
	} else {
		localBL = blacklist.New(cfg.CleanupInterval)
		authService = auth.New(log, userStore, localBL, cfg.TokenTTL, cfg.TokenSecret)
		tokenChecker = localBL
	}

	if cfg.CatalogGRPCAddr != "" {
		cl, err := catalogservice.NewClient(cfg.CatalogGRPCAddr)
		if err != nil {
			log.Error("failed to connect to catalog gRPC service", "addr", cfg.CatalogGRPCAddr, "err", err)
			panic(err)
		}
		catalogCl = cl
		adsService = catalogservice.NewAdsClient(cl)
		favService = cl
		log.Info("connected to catalog-service via gRPC", slog.String("addr", cfg.CatalogGRPCAddr))
	}

	services := httpapp.Services{
		Ads:       adsService,
		Auth:      authService,
		Cart:      cartService,
		Favorites: favService,
		Wallet:    walletSvc,
	}

	httpApp := httpapp.New(log, services, tokenChecker, cfg.HTTP.Port, cfg.TokenTTL, cfg.TokenSecret)

	return &App{
		HTTPServer:    httpApp,
		Blacklist:     localBL,
		DB:            pgDB,
		authClient:    authCl,
		catalogClient: catalogCl,
	}
}

func (a *App) Stop() {
	if a.Blacklist != nil {
		a.Blacklist.Stop()
	}
	a.HTTPServer.Stop()
	if a.authClient != nil {
		_ = a.authClient.Close()
	}
	if a.catalogClient != nil {
		_ = a.catalogClient.Close()
	}
	if a.DB != nil {
		_ = a.DB.Close()
	}
}
