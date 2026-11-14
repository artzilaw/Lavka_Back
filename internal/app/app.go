package app

import (
	"log/slog"

	httpapp "lavka/internal/app/http"
	"lavka/internal/config"
	"lavka/internal/services/auth"
	"lavka/internal/storage"
	"lavka/internal/storage/ads"
	"lavka/internal/storage/blacklist"
)

type App struct {
	HTTPServer *httpapp.App
	Blacklist  *blacklist.InMemory
}

func New(log *slog.Logger, cfg *config.Config) *App {
	st, err := storage.New(cfg.StoragePath)
	if err != nil {
		log.Error("failed to initialize storage", "err", err)
		panic(err)
	}

	tokenBlacklist := blacklist.New(cfg.CleanupInterval)
	authService := auth.New(log, st, tokenBlacklist, cfg.TokenTTL, cfg.TokenSecret)
	adsService := ads.NewAdsRepository()

	services := httpapp.Services{
		Ads:  adsService,
		Auth: authService,
	}

	httpApp := httpapp.New(log, services, tokenBlacklist, cfg.HTTP.Port, cfg.TokenTTL, cfg.TokenSecret)

	return &App{
		HTTPServer: httpApp,
		Blacklist:  tokenBlacklist,
	}
}

func (a *App) Stop() {
	a.Blacklist.Stop()
	a.HTTPServer.Stop()
}
