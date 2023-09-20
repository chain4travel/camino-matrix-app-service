package cmd

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/chain4travel/camino-synapse-app-service/internal/app"
	"github.com/chain4travel/camino-synapse-app-service/internal/config"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func Execute() error {
	rootCmd := &cobra.Command{
		Use: "app-service",
		Run: func(cmd *cobra.Command, args []string) {
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
				return
			}

			if cfg.LogLevel == "info" {
				zapLogger, err = zap.NewProduction()
				if err != nil {
					log.Fatal(err)
				}
				_ = logger.Sync()
				logger = zapLogger.Sugar()
				defer func() { _ = logger.Sync() }()
			}

			app, err := app.NewApp(ctx, logger, cfg)
			if err != nil {
				app.Close(context.Background())
				return
			}

			app.Run(ctx)
		},
	}
	config.BindFlags(rootCmd)
	return rootCmd.Execute()
}
