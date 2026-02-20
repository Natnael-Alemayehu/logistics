package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/natnael-alemayehu/logistics/internal/config"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/pkg/queue"
	"github.com/natnael-alemayehu/logistics/pkg/redis"
	"github.com/rs/zerolog"
)

func main() {
	cfg := config.Load()

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if cfg.Environment == "development" {
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}
	logger.Info().Msg("Starting logistics worker")

	if cfg.RedisURL == "" {
		logger.Fatal().Msg("Redis URL is required for worker")
	}

	redisClient, err := redis.NewClient(redis.Config{
		Addr:     cfg.RedisURL,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer redisClient.Close()

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer pool.Close()

	redisQueue := queue.NewRedisQueue(redisClient, "logistics:queue")
	worker := queue.NewWorker(redisQueue, 5, logger)

	worker.RegisterHandler(queue.JobTypePhotoCompress, func(ctx context.Context, job *queue.Job) error {
		// Example compression logic stub
		logger.Info().Str("job_id", job.ID).Msg("Compressing photo (stub)")
		return nil
	})

	worker.RegisterHandler(queue.JobTypeSMSBatch, func(ctx context.Context, job *queue.Job) error {
		// Example SMS batch logic stub
		logger.Info().Str("job_id", job.ID).Msg("Sending batch SMS (stub)")
		return nil
	})

	worker.Start()
	logger.Info().Msg("Worker started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down worker...")
	worker.Stop()
	logger.Info().Msg("Worker stopped")
}
