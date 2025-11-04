// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/chain4travel/camino-matrix-app-service/config"
	"github.com/chain4travel/camino-matrix-app-service/internal/service"
	service_storage "github.com/chain4travel/camino-matrix-app-service/internal/storage/sqlite"
	"github.com/chain4travel/camino-messenger-bot/v11/pkg/chequehandler"
	cheque_handler_storage "github.com/chain4travel/camino-messenger-bot/v11/pkg/chequehandler/storage/sqlite"
	cmaccounts "github.com/chain4travel/camino-messenger-bot/v11/pkg/cm_accounts"
	"github.com/chain4travel/camino-messenger-bot/v11/pkg/scheduler"
	scheduler_storage "github.com/chain4travel/camino-messenger-bot/v11/pkg/scheduler/storage/sqlite"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jonboulle/clockwork"
	"go.uber.org/zap"

	"golang.org/x/sync/errgroup"
)

const (
	cashInJobName        = "cash_in"
	cmAccountsCacheSize  = 1000
	cashInTxIssueTimeout = 10 * time.Second
)

func NewApp(ctx context.Context, logger *zap.SugaredLogger, cfg *config.Config) (*App, error) {
	ethClient, err := ethclient.Dial(cfg.ChainRPCURL)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	cmAccounts, err := cmaccounts.NewService(ctx, logger, cmAccountsCacheSize, ethClient)
	if err != nil {
		logger.Errorf("Failed to create cm accounts service: %v", err)
		return nil, err
	}

	chequeHandlerStorage, err := cheque_handler_storage.New(
		ctx,
		logger,
		cfg.DB.ChequeHandler.DBPath,
	)
	if err != nil {
		logger.Errorf("Failed to create cheque handler storage: %v", err)
		return nil, err
	}

	chequeHandler, err := chequehandler.NewChequeHandler(
		ctx,
		logger,
		ethClient,
		cfg.NetworkFeeRecipientBotKey,
		cfg.NetworkFeeRecipientCMAccountAddress,
		chainID,
		chequeHandlerStorage,
		cmAccounts,
		cfg.MinChequeDurationUntilExpiration,
		nil, // ChequeExpirationTime is not used, because no cheques will be issued
		cashInTxIssueTimeout,
	)
	if err != nil {
		logger.Errorf("Failed to create cheque handler: %v", err)
		return nil, err
	}

	schedulerStorage, err := scheduler_storage.New(ctx, logger, cfg.DB.Scheduler.DBPath)
	if err != nil {
		logger.Errorf("Failed to create storage: %v", err)
		return nil, err
	}

	scheduler := scheduler.New(logger, schedulerStorage, clockwork.NewRealClock())

	serviceStorage, err := service_storage.New(
		ctx,
		logger,
		cfg.DB.Service.DBPath,
	)
	if err != nil {
		logger.Errorf("Failed to create service storage: %v", err)
		return nil, err
	}

	service := service.NewService(
		logger,
		cfg.NetworkFeeRecipientCMAccountAddress,
		serviceStorage,
		ethClient,
		chainID,
		chequeHandler,
		cmAccounts,
	)

	return &App{
		cfg:           cfg,
		logger:        logger,
		scheduler:     scheduler,
		service:       service,
		chequeHandler: chequeHandler,
		storage:       serviceStorage,
		httpServer:    newServer(logger, cfg.Matrix.AccessToken, cfg.Matrix.HTTPPort, service),
	}, nil
}

type App struct {
	logger        *zap.SugaredLogger
	cfg           *config.Config
	service       service.Service
	scheduler     scheduler.Scheduler
	chequeHandler chequehandler.ChequeHandler
	httpServer    *server
	storage       service_storage.Storage
}

func (a *App) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx) // error here will call gCtx.cancel() and finish other Go-s

	// run

	cashInStatusCheckDone := make(chan struct{})
	schedulerStarted := make(chan struct{})

	a.safeGo(g, func() error {
		a.logger.Info("Starting start-up cash-in status check...")
		if err := a.chequeHandler.CheckCashInStatus(ctx); err != nil {
			return fmt.Errorf("failed to do start-up cash-in status check: %w", err)
		}
		a.logger.Info("Start-up cash-in status check done.")
		close(cashInStatusCheckDone)
		return nil
	})

	a.safeGo(g, func() error {
		if !awaitChan(ctx, cashInStatusCheckDone) {
			return nil
		}
		a.logger.Info("Starting scheduler...")

		a.scheduler.RegisterJobHandler(cashInJobName, func() {
			if err := a.chequeHandler.CashIn(ctx); err != nil && !errors.Is(err, context.Canceled) {
				a.logger.Errorf("Failed to do scheduled cash in: %v", err)
				return
			}
		})

		if err := a.scheduler.Schedule(ctx, a.cfg.CashInPeriod, cashInJobName); err != nil {
			return fmt.Errorf("failed to schedule cash in job: %w", err)
		}
		if err := a.scheduler.Start(ctx); err != nil {
			return fmt.Errorf("failed to start scheduler: %w", err)
		}

		a.logger.Info("Scheduler started.")
		close(schedulerStarted)

		return nil
	})

	a.safeGo(g, func() error {
		if !awaitChans(ctx,
			cashInStatusCheckDone,
			schedulerStarted,
		) {
			return nil
		}

		a.logger.Info("Starting HTTP server...")
		errChan := a.httpServer.Start()
		a.logger.Info("HTTP server started.")

		if err := <-errChan; err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Errorf("HTTP server stopped with error: %v", err)
			return err
		}
		return nil
	})

	// stop

	a.safeGo(g, func() error {
		<-ctx.Done()
		a.logger.Debug("Stopping HTTP server...")
		// we use background context, because we want to try to shutdown HTTP server gracefully regardless
		if err := a.httpServer.Stop(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
			a.logger.Errorf("Failed to stop HTTP server: %v", err)
			return fmt.Errorf("failed to stop HTTP server: %w", err)
		}
		a.logger.Debug("HTTP server stopped.")
		return nil
	})

	a.safeGo(g, func() error {
		<-ctx.Done()
		a.logger.Debug("Closing storage...")
		if err := a.storage.Close(); err != nil {
			a.logger.Errorf("Failed to close storage: %v", err)
			return fmt.Errorf("failed to close storage: %w", err)
		}
		a.logger.Debug("Storage closed.")
		return nil
	})

	a.safeGo(g, func() error {
		<-ctx.Done()
		a.logger.Debug("Stopping scheduler...")
		a.scheduler.Stop()
		a.logger.Debug("Scheduler stopped.")
		return nil
	})

	// wait
	err := g.Wait()
	if err != nil {
		a.logger.Error(err) // will log first run/stop error
	}

	return err
}

func (a *App) safeGo(g *errgroup.Group, fn func() error) {
	g.Go(func() (err error) {
		defer func() {
			if panicErr := recover(); panicErr != nil {
				err = fmt.Errorf("panic: %v", panicErr) // err will be returned
				a.logger.Errorf("recovered from panic: %v", err)
			}
		}()
		return fn()
	})
}

func awaitChan(ctx context.Context, ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	case <-ctx.Done():
		return false
	}
}

func awaitChans(ctx context.Context, chans ...<-chan struct{}) bool {
	for _, ch := range chans {
		if !awaitChan(ctx, ch) {
			return false
		}
	}
	return true
}
