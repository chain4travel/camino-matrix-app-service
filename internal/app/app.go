package app

import (
	"camino-synapse-appservice/internal/config"
	"camino-synapse-appservice/internal/matrix"
	"camino-synapse-appservice/internal/node"
	"camino-synapse-appservice/internal/scheduler"
	"camino-synapse-appservice/internal/service"
	"camino-synapse-appservice/internal/storage"
	"context"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const caminoUserID = "@camino:localhost:8080" // TODO leave camino part, reconstruct the rest

func NewApp(ctx context.Context, logger *zap.SugaredLogger, cfg *config.Config) (*app, error) {
	logger.Debug("Creating matrix client...")
	matrixClient, err := matrix.NewClient(ctx, logger, cfg.MatrixHost, cfg.AccessToken, caminoUserID)
	if err != nil {
		return nil, err
	}

	logger.Debug("Creating camino node client...")
	nodeClient, err := node.NewClient(ctx, cfg.CaminoNodeHost, logger)
	if err != nil {
		return nil, err
	}

	logger.Debug("Creating storage...")
	storage, err := storage.New(ctx, logger, cfg.DBPath)
	if err != nil {
		return nil, err
	}

	logger.Debug("Creating scheduler...")
	scheduler := scheduler.New(ctx)

	logger.Debug("Creating service...")
	service := service.New(ctx, logger, storage, nodeClient, matrixClient)

	logger.Debug("Creating HTTP server...")
	httpServer := newServer(ctx, logger, service, cfg.MatrixAccessToken)

	return &app{
		logger:       logger,
		cfg:          cfg,
		matrixClient: matrixClient,
		nodeClient:   nodeClient,
		httpServer:   httpServer,
		service:      service,
		scheduler:    scheduler,
		storage:      storage,
	}, nil
}

type app struct {
	logger       *zap.SugaredLogger
	cfg          *config.Config
	matrixClient matrix.Client
	nodeClient   node.Client
	httpServer   *server
	service      *service.Service
	storage      storage.Storage
	scheduler    scheduler.Scheduler
}

func (app *app) Run(ctx context.Context) {
	g, gCtx := errgroup.WithContext(ctx) // error here will call ctx.cancel() and finish other Go-s

	// run

	g.Go(func() error {
		app.logger.Debug("Scheduling cashOutTxs-issuing...")
		if err := app.scheduler.Shedule(app.cfg.CashOutPeriod, app.service.CashOut); err != nil {
			app.logger.Error(err)
			return err
		}
		app.logger.Debug("Scheduling cashOutTxs-checking...")
		if err := app.scheduler.Shedule(app.cfg.CashOutTxCheckPeriod, app.service.CheckCashOutTxs); err != nil {
			app.logger.Error(err)
			return err
		}

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

	if err := g.Wait(); err != nil {
		app.logger.Error(err) // will log first run/stop error
	}
}

func (app *app) Close(ctx context.Context) {
	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		app.logger.Debug("Stopping HTTP server...")
		return app.httpServer.Stop(ctx)
	})

	g.Go(func() error {
		app.logger.Debug("Stopping scheduler...")
		return app.scheduler.Stop(ctx)
	})

	g.Go(func() error {
		app.logger.Debug("Closing storage...")
		return app.storage.Close(ctx)
	})

	if err := g.Wait(); err != nil {
		app.logger.Error(err)
	}
}
