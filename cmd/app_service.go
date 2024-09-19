package cmd

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/chain4travel/camino-synapse-app-service/internal/app"
	"github.com/chain4travel/camino-synapse-app-service/internal/config"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	Version = "0.0.1"
)

var rootCmd = &cobra.Command{
	Use:        "camino-matrix-app-service",
	Short:      "starts camino matrix app-service",
	Version:    Version,
	SuggestFor: []string{"camino-matrix", "matrix-app-service", "camino-app-service", "app-service"},
	RunE:       rootFunc,
}

func rootFunc(_ *cobra.Command, _ []string) error {
	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal(err)
	}
	logger := zapLogger.Sugar()
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.ReadConfig(ctx, logger)
	if err != nil {
		logger.Error(err)
		return err
	}

	if cfg.LogLevel == "info" {
		zapLogger, err = zap.NewProduction()
		if err != nil {
			logger.Error(err)
			return err
		}
		_ = logger.Sync()
		logger = zapLogger.Sugar()
		defer func() { _ = logger.Sync() }()
	}

	app, err := app.NewApp(ctx, logger, cfg)
	if err != nil {
		logger.Error(err)
		app.Close(context.Background())
		return err
	}

	return app.Run(ctx)
}

func init() {
	cobra.EnablePrefixMatching = true

	if err := config.BindFlags(rootCmd); err != nil {
		panic(fmt.Errorf("failed to bind flags: %w", err))
	}
}

func Execute() error {
	return rootCmd.Execute()
}
