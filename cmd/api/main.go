package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/config"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/handler"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/jwt"
	"github.com/natnael-alemayehu/logistics/pkg/validation"
	"github.com/rs/zerolog"
)

func main() {
	cfg := config.Load()

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if cfg.Environment == "development" {
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}
	logger.Info().Msg("Starting logistics API")

	validation.Init()

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer pool.Close()
	logger.Info().Msg("Connected to database")

	privateKey, err := jwt.LoadPrivateKey(cfg.JWTPrivateKeyPath)
	if err != nil {
		logger.Warn().Msg("Generating new JWT key pair")
		if err := jwt.GenerateKeyPair(cfg.JWTPrivateKeyPath, cfg.JWTPublicKeyPath); err != nil {
			logger.Fatal().Err(err).Msg("Failed to generate JWT keys")
		}
		privateKey, err = jwt.LoadPrivateKey(cfg.JWTPrivateKeyPath)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to load private key after generation")
		}
	}

	publicKey, err := jwt.LoadPublicKey(cfg.JWTPublicKeyPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load public key")
	}

	jwtManager := jwt.NewManager(privateKey, publicKey, cfg.JWTIssuer, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)

	queries := db.New(pool)
	authService := service.NewAuthService(queries, jwtManager)
	shipmentService := service.NewShipmentService(queries)
	syncService := service.NewSyncService(queries, shipmentService)
	userService := service.NewUserService(queries)

	h := handler.New(authService, shipmentService, syncService, userService)

	router := h.Routes(logger, jwtManager)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server error")
		}
	}()

	logger.Info().Str("port", cfg.Port).Msg("Server started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown error")
	}

	logger.Info().Msg("Server stopped")
}
