package app

import (
	"context"

	"github.com/chain4travel/camino-matrix-app-service/config"
	"github.com/chain4travel/camino-matrix-app-service/internal/service"
	"github.com/chain4travel/camino-matrix-app-service/internal/storage"
	"github.com/chain4travel/camino-messenger-bot/pkg/database/sqlite"
	"github.com/chain4travel/camino-messenger-bot/pkg/scheduler"
	scheduler_storage "github.com/chain4travel/camino-messenger-bot/pkg/scheduler/storage/sqlite"
	"github.com/jonboulle/clockwork"
	"go.uber.org/zap"

	"golang.org/x/sync/errgroup"
)

const cashInJobName = "cash_in"

func NewApp(ctx context.Context, logger *zap.SugaredLogger, cfg *config.Config) (*App, error) {
	app := &App{
		logger: logger,
		cfg:    cfg,
	}

	logger.Debug("Creating service...")
	service, err := service.NewService(
		ctx,
		logger,
		cfg.CChainRPCURL,
		cfg.CMAccountAddress,
		cfg.NetworkFeeRecipientKey,
		cfg.MinDurationUntilExpiration,
		nil, // TODO@
	)
	if err != nil {
		return app, err
	}
	app.service = service

	logger.Debug("Creating HTTP server...")
	app.httpServer = newServer(ctx, logger, cfg.MatrixAccessToken, cfg.HTTPPort, service)

	storage, err := scheduler_storage.New(ctx, logger, sqlite.DBConfig(cfg.DB.Scheduler))
	if err != nil {
		logger.Errorf("Failed to create storage: %v", err)
		return nil, err
	}

	logger.Debug("Creating scheduler...")
	app.scheduler = scheduler.New(logger, storage, clockwork.NewRealClock())
	app.scheduler.RegisterJobHandler(cashInJobName, func() {
		_ = service.CashIn(context.Background())
	})
	if err := app.scheduler.Schedule(ctx, cfg.CashInPeriod, cashInJobName); err != nil {
		logger.Errorf("failed to schedule job %s: %v", cashInJobName, err)
		return nil, err
	}

	return app, nil
}

type App struct {
	logger     *zap.SugaredLogger
	cfg        *config.Config
	service    service.Service
	scheduler  scheduler.Scheduler
	httpServer *server
	storage    storage.Storage
}

func (app *App) Run(ctx context.Context) error {
	g, gCtx := errgroup.WithContext(ctx) // error here will call ctx.cancel() and finish other Go-s

	// run

	g.Go(func() error {
		return app.httpServer.Start(gCtx)
	})

	g.Go(func() error {
		return app.service.CheckCashInStatus(gCtx)
	})

	g.Go(func() error {
		return app.scheduler.Start(gCtx)
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
		app.logger.Debug("Closing storage...")
		return app.storage.Close(context.Background())
	})

	g.Go(func() error {
		<-gCtx.Done()
		app.logger.Debug("Stopping scheduler...")
		return app.scheduler.Stop()
	})

	// wait
	err := g.Wait()
	if err != nil {
		app.logger.Error(err) // will log first run/stop error
	}

	return err
}

func (app *App) Close(ctx context.Context) {
	g, _ := errgroup.WithContext(ctx)

	if app.httpServer != nil {
		g.Go(func() error {
			app.logger.Debug("Stopping HTTP server...")
			return app.httpServer.Stop(ctx)
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
