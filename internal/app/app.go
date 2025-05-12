// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package app

import (
	"context"
	"fmt"
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

	cmAccounts, err := cmaccounts.NewService(logger, cmAccountsCacheSize, ethClient)
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
	scheduler.RegisterJobHandler(cashInJobName, func() {
		_ = chequeHandler.CashIn(context.Background())
	})

	serviceStorage, err := service_storage.New(
		ctx,
		logger,
		cfg.DB.ChequeHandler.DBPath,
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
		cmAccounts,
	)

	return &App{
		cfg:           cfg,
		logger:        logger,
		scheduler:     scheduler,
		service:       service,
		chequeHandler: chequeHandler,
		storage:       serviceStorage,
		httpServer:    newServer(ctx, logger, cfg.Matrix.AccessToken, cfg.Matrix.HTTPPort, service),
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
	g, gCtx := errgroup.WithContext(ctx) // error here will call gCtx.cancel() and finish other Go-s

	// run

	cashInStatusCheckDone := make(chan struct{})
	schedulerStarted := make(chan struct{})

	g.Go(func() error {
		a.logger.Info("Starting start-up cash-in status check...")
		if err := a.chequeHandler.CheckCashInStatus(gCtx); err != nil {
			return fmt.Errorf("failed to check start-up cash-in status: %w", err)
		}
		a.logger.Info("Start-up cash-in status check done.")
		close(cashInStatusCheckDone)
		return nil
	})

	g.Go(func() error {
		if !awaitChan(gCtx, cashInStatusCheckDone) {
			return nil
		}

		a.logger.Info("Starting scheduler...")

		if err := a.scheduler.Schedule(gCtx, a.cfg.CashInPeriod, cashInJobName); err != nil {
			return fmt.Errorf("failed to schedule cash in job: %w", err)
		}
		if err := a.scheduler.Start(gCtx); err != nil {
			return fmt.Errorf("failed to start scheduler: %w", err)
		}

		a.logger.Info("Scheduler started.")
		close(schedulerStarted)
		return nil
	})

	g.Go(func() error {
		if !awaitChans(gCtx,
			cashInStatusCheckDone,
			schedulerStarted,
		) {
			return nil
		}

		a.logger.Info("Starting http server...")
		return a.httpServer.Start(gCtx)
	})

	// stop

	g.Go(func() error {
		<-gCtx.Done()
		a.logger.Debug("Stopping HTTP server...")
		return a.httpServer.Stop(context.Background())
	})

	g.Go(func() error {
		<-gCtx.Done()
		a.logger.Debug("Closing storage...")
		return a.storage.Close()
	})

	g.Go(func() error {
		<-gCtx.Done()
		a.logger.Debug("Stopping scheduler...")
		return a.scheduler.Stop()
	})

	// wait
	err := g.Wait()
	if err != nil {
		a.logger.Error(err) // will log first run/stop error
	}

	return err
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
