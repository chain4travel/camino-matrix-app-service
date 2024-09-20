package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/chain4travel/camino-matrix-app-service/internal/logger"
	"github.com/chain4travel/camino-matrix-app-service/internal/service"
	"maunium.net/go/mautrix/event"

	"github.com/gin-gonic/gin"
)

func newServer(_ context.Context, logger logger.Logger, hsAccessToken, port string, service service.Service) *server {
	ginRouter := gin.New()
	s := &server{
		logger: logger,
		gin:    ginRouter,
		port:   port,
		httpServer: &http.Server{
			Addr:              ":" + port,
			Handler:           ginRouter,
			ReadHeaderTimeout: time.Second * 10,
		},
		service: service,
	}

	s.gin.Use(middlewareLogger(logger))
	s.gin.Use(middlewareRecover(logger))
	s.gin.Use(middlewareBearerAuth(hsAccessToken))

	s.gin.PUT("/_matrix/app/v1/transactions/:txnId", s.putTransactions)
	s.gin.POST("/_matrix/app/v1/ping", s.ping)

	return s
}

type server struct {
	logger     logger.Logger
	httpServer *http.Server
	gin        *gin.Engine
	port       string
	service    service.Service
}

func (s *server) Start(_ context.Context) error {
	s.logger.Infof("Started HTTP server on %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func (s *server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// handlers

type putTransactionsReq struct {
	Events []event.Event `json:"events"`
}

func (s *server) putTransactions(c *gin.Context) {
	reqBody := &putTransactionsReq{}
	if err := c.Bind(reqBody); err != nil {
		s.logger.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}

	if err := s.service.ProcessEvents(c.Request.Context(), reqBody.Events); err != nil {
		s.logger.Errorf("failed to process events: %v", err) // TODO @evlekht add transaction ID to log
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func (s *server) ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}

// middlewares

func middlewareBearerAuth(token string) gin.HandlerFunc {
	const bearer = "Bearer"
	const l = len(bearer)

	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if len(auth) > l+1 && strings.EqualFold(auth[:l], bearer) && auth[l+1:] == token {
			c.Next()
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	}
}

func middlewareRecover(logger logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("panic recovered: %v", r)
			}
		}()
		c.Next()
	}
}

func middlewareLogger(logger logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug(c.Request.Method, c.Request.URL.Path)
		c.Next()
	}
}
