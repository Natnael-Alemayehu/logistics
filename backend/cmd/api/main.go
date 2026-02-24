package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/natnael-alemayehu/logistics/docs"
	"github.com/natnael-alemayehu/logistics/internal/config"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/handler"
	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/jwt"
	"github.com/natnael-alemayehu/logistics/pkg/redis"
	"github.com/natnael-alemayehu/logistics/pkg/validation"
	"github.com/natnael-alemayehu/logistics/pkg/websocket"
	"github.com/rs/zerolog"
)

// @title Ethiopian Logistics Tracking Platform API
// @version 1.0
// @description API for managing logistics operations including shipments, drivers, vehicles, and tracking
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@logistics.et

// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
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

	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		var err error
		redisClient, err = redis.NewClient(redis.Config{
			Addr:     cfg.RedisURL,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to connect to Redis, continuing without caching")
		} else {
			defer redisClient.Close()
			logger.Info().Msg("Connected to Redis")
		}
	}

	queries := db.New(pool)
	auditService := service.NewAuditService(queries)
	authService := service.NewAuthService(queries, jwtManager, auditService)

	wsHub := websocket.NewHub(logger)
	go wsHub.Run()

	eventService := service.NewEventService(wsHub, logger)

	shipmentService := service.NewShipmentService(queries, auditService, nil, eventService)
	syncService := service.NewSyncService(queries, shipmentService, eventService)
	userService := service.NewUserService(queries, auditService)
	vehicleService := service.NewVehicleService(queries)
	trackingService := service.NewTrackingService(queries, auditService)
	driverService := service.NewDriverService(queries, pool)
	deviceTokenService := service.NewDeviceTokenService(queries)

	wsHandler := handler.NewWSHandler(wsHub, jwtManager)
	dashboardService := handler.NewDashboardService(queries)

	h := handler.New(authService, shipmentService, syncService, userService, vehicleService, trackingService, wsHandler, dashboardService, driverService, deviceTokenService)

	var rateLimiter *middleware.RateLimiter
	if redisClient != nil {
		rateLimiter = middleware.NewRateLimiter(redisClient, cfg.RateLimitIP, cfg.RateLimitUser, cfg.RateLimitAuth)
	}

	router := h.Routes(logger, jwtManager, rateLimiter)

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
