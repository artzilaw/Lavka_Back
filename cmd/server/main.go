package main

import (
	"os"
	"os/signal"
	"syscall"

	"lavka/internal/app"
	"lavka/internal/config"
	"lavka/internal/logger"
)

func main() {
	cfg := config.MustLoadConfig()
	log := logger.SetupLogger(cfg.Env)

	log.Info("starting lavka backend")

	application := app.New(log, cfg)
	go application.HTTPServer.MustRun()
	log.Info("lavka backend started")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	sign := <-stop
	log.Info("received signal", "signal", sign)

	log.Info("stopping lavka backend")
	application.Stop()
	log.Info("lavka backend stopped")
}
