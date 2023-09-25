package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/chain4travel/camino-synapse-app-service/internal/service"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/gommon/log"
	"go.uber.org/zap"
	"maunium.net/go/mautrix/event"
)

func newServer(ctx context.Context, logger *zap.SugaredLogger, service *service.Service, hsAccessToken, port string) *server {
	s := &server{
		logger:  logger,
		e:       echo.New(),
		service: service,
		port:    port,
	}

	s.e.HideBanner = true
	s.e.Logger.SetLevel(log.DEBUG)

	s.e.Use(middleware.Logger())
	s.e.Use(middlewareRecover(logger))
	s.e.Use(middlewareBearerAuth(hsAccessToken))

	s.e.PUT("/_matrix/app/v1/transactions/:txnId", s.putTransactions)
	s.e.POST("/_matrix/app/v1/ping", s.ping)

	return s
}

type server struct {
	logger  *zap.SugaredLogger
	e       *echo.Echo
	service *service.Service
	port    string
}

func (s *server) Start(ctx context.Context) error {
	return s.e.Start(":" + s.port)
}

func (s *server) Stop(ctx context.Context) error {
	return s.e.Shutdown(ctx)
}

// handlers

type putTransacitonsReq struct {
	Events []event.Event `json:"events"`
}

func (s *server) putTransactions(c echo.Context) error {
	reqBody := &putTransacitonsReq{}
	if err := c.Bind(reqBody); err != nil {
		s.logger.Error(err)
		return err
	}

	if err := s.service.ProcessEvents(c.Request().Context(), reqBody.Events, c.Param("txnId")); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, struct{}{})
}

func (s *server) ping(c echo.Context) error {
	return c.JSON(http.StatusOK, struct{}{})
}

// middlewares

func middlewareBearerAuth(token string) echo.MiddlewareFunc {
	const bearer = "Bearer"
	const l = len(bearer)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth := c.Request().Header.Get(echo.HeaderAuthorization)
			if len(auth) > l+1 && strings.EqualFold(auth[:l], bearer) && auth[l+1:] == token {
				return next(c)
			}
			return echo.ErrUnauthorized
		}
	}
}

func middlewareRecover(logger *zap.SugaredLogger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("%v", r)
					}
					logger.Errorf("PANIC recovered: %v", err)
				}
			}()
			return next(c)
		}
	}
}
