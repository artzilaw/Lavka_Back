package app

import (
	"context"
	"database/sql"
	"log/slog"
	"path/filepath"

	httpapp "lavka/internal/app/http"
	"lavka/internal/config"
	"lavka/internal/services/auth"
	"lavka/internal/storage"
	"lavka/internal/storage/ads"
	"lavka/internal/storage/blacklist"
	"lavka/internal/storage/cart"
	"lavka/internal/storage/postgres"
)

type App struct {
	HTTPServer *httpapp.App
	Blacklist  *blacklist.InMemory
	DB         *sql.DB
}

func New(log *slog.Logger, cfg *config.Config) *App {
	var (
		userStore   auth.UserProviderSaver
		adsService  httpapp.Ads
		cartService httpapp.Cart
		pgDB        *sql.DB
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
		log.Info("using local JSON/in-memory storage (DATABASE_URL not set)")
	}

	tokenBlacklist := blacklist.New(cfg.CleanupInterval)
	authService := auth.New(log, userStore, tokenBlacklist, cfg.TokenTTL, cfg.TokenSecret)

	services := httpapp.Services{
		Ads:  adsService,
		Auth: authService,
		Cart: cartService,
	}

	httpApp := httpapp.New(log, services, tokenBlacklist, cfg.HTTP.Port, cfg.TokenTTL, cfg.TokenSecret)

	return &App{
		HTTPServer: httpApp,
		Blacklist:  tokenBlacklist,
		DB:         pgDB,
	}
}

func (a *App) Stop() {
	a.Blacklist.Stop()
	a.HTTPServer.Stop()
	if a.DB != nil {
		_ = a.DB.Close()
	}
}
