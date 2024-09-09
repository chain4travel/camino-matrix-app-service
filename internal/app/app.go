package app

import (
	"context"

	"github.com/chain4travel/camino-synapse-app-service/internal/config"
	"github.com/chain4travel/camino-synapse-app-service/internal/logger"
	"github.com/chain4travel/camino-synapse-app-service/internal/matrix"
	"github.com/chain4travel/camino-synapse-app-service/internal/node"
	"github.com/chain4travel/camino-synapse-app-service/internal/scheduler"
	"github.com/chain4travel/camino-synapse-app-service/internal/service"
	"github.com/chain4travel/camino-synapse-app-service/internal/storage"

	"golang.org/x/sync/errgroup"
)

const caminoUserID = "@camino:localhost:8080" // TODO leave camino part, reconstruct the rest

func NewApp(ctx context.Context, logger logger.Logger, cfg *config.Config) (*app, error) {
	app := &app{
		logger: logger,
		cfg:    cfg,
	}

	logger.Debug("Creating camino node client...")
	nodeClient, err := node.NewClient(ctx, cfg.CaminoNodeHost, logger)
	if err != nil {
		return app, err
	}
	app.nodeClient = nodeClient

	logger.Debug("Creating matrix client...")
	matrixClient, err := matrix.NewClient(ctx, logger, cfg.MatrixHost, cfg.AccessToken, caminoUserID, nodeClient.NetworkID())
	if err != nil {
		return app, err
	}
	app.matrixClient = matrixClient

	logger.Debug("Creating storage...")
	storage, err := storage.New(ctx, logger, cfg.DBPath, cfg.DBName, cfg.MigrationsPath)
	if err != nil {
		return app, err
	}
	app.storage = storage

	logger.Debug("Creating scheduler...")
	app.scheduler = scheduler.New(ctx)

	logger.Debug("Creating service...")
	app.service = service.New(ctx, logger, storage, nodeClient, matrixClient)

	logger.Debug("Creating HTTP server...")
	app.httpServer = newServer(ctx, logger, app.service, cfg.MatrixAccessToken, cfg.HTTPPort)

	return app, nil
}

type app struct {
	logger       logger.Logger
	cfg          *config.Config
	matrixClient matrix.Client
	nodeClient   node.Client
	httpServer   *server
	service      *service.Service
	storage      storage.Storage
	scheduler    scheduler.Scheduler
}

func (app *app) Run(ctx context.Context) error {
	g, gCtx := errgroup.WithContext(ctx) // error here will call ctx.cancel() and finish other Go-s

	// run

	g.Go(func() error {
		app.logger.Debug("Scheduling cash-out...")
		// if err := app.scheduler.Schedule(app.cfg.CashOutPeriod, app.service.CashOut); err != nil {
		// 	app.logger.Error(err)
		// 	return err
		// }

		app.logger.Debug("Starting scheduler...")
		// scheduler.Start doesn't block, but we need group to call gCtx.cancel if this one errors
		return app.scheduler.Start(ctx)
	})

	g.Go(func() error {
		app.logger.Debug("Starting HTTP server...")
		return app.httpServer.Start(ctx)
	})

	// stop
	// <-gCtx.Done() means that all "run" goroutines are finished

	g.Go(func() error {
		<-gCtx.Done()
		app.logger.Debug("Stopping HTTP server...")
		return app.httpServer.Stop(context.Background())
	})

	g.Go(func() error {
		<-gCtx.Done()
		app.logger.Debug("Stopping scheduler...")
		return app.scheduler.Stop(context.Background())
	})

	g.Go(func() error {
		<-gCtx.Done()
		app.logger.Debug("Closing storage...")
		return app.storage.Close(context.Background())
	})

	// wait
	err := g.Wait()
	if err != nil {
		app.logger.Error(err) // will log first run/stop error
	}

	return err
}

func (app *app) Close(ctx context.Context) {
	g, _ := errgroup.WithContext(ctx)

	if app.httpServer != nil {
		g.Go(func() error {
			app.logger.Debug("Stopping HTTP server...")
			return app.httpServer.Stop(ctx)
		})
	}

	if app.scheduler != nil {
		g.Go(func() error {
			app.logger.Debug("Stopping scheduler...")
			return app.scheduler.Stop(ctx)
		})
	}

	if app.storage != nil {
		g.Go(func() error {
			app.logger.Debug("Closing storage...")
			return app.storage.Close(ctx)
		})
	}

	if err := g.Wait(); err != nil {
		app.logger.Error(err)
	}
}
