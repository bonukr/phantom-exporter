// Command exporter runs the phantom-exporter HTTP service (API, GUI, /metrics).
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/bonukr/phantom-exporter/internal/config"
	"github.com/bonukr/phantom-exporter/internal/logging"
	"github.com/bonukr/phantom-exporter/internal/metrics"
	"github.com/bonukr/phantom-exporter/internal/server"
	"github.com/bonukr/phantom-exporter/internal/store"
	"github.com/bonukr/phantom-exporter/web"
)

func main() {
	cfg := config.Load()

	logger, closer, err := logging.New(cfg.LogFile, cfg.LogLevel)
	if err != nil {
		panic(err)
	}
	defer closer.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, cfg.SettingsDir, logger)
	if err != nil {
		logger.Error("failed to init store", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	gen := metrics.NewGenerator()
	srv := server.New(st, gen, logger, web.Assets())

	if err := srv.Run(ctx, ":"+cfg.Port); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
	logger.Info("shutdown complete")
}
