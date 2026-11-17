// Command server runs Kiflet: the dispatcher dashboard, public tracking, the
// driver app and its API, and the background jobs, in one process.
//
//	server                      serve (the default)
//	server migrate              apply database migrations and exit
//	server seed                 load demo data into an empty database
//	server sms-in <phone> <text>  text the running server, as a phone would
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"

	"github.com/natnael-alemayehu/kiflet/internal/app"
	"github.com/natnael-alemayehu/kiflet/internal/config"
	"github.com/natnael-alemayehu/kiflet/migrations"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration:", err)
		os.Exit(1)
	}
	log := newLogger(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "serve":
		err = serve(ctx, cfg, log)
	case "migrate":
		err = withPool(ctx, cfg, log, func(pool *pgxpool.Pool) error { return migrate(ctx, pool, log) })
	case "seed":
		err = withPool(ctx, cfg, log, func(pool *pgxpool.Pool) error {
			if err := migrate(ctx, pool, log); err != nil {
				return err
			}
			return seed(ctx, cfg, pool, log)
		})
	case "sms-in":
		err = smsIn(ctx, cfg, os.Args[2:])
	case "help", "-h", "--help":
		fmt.Println("usage: server [serve | migrate | seed | sms-in <phone> <text>]")
		return
	default:
		err = fmt.Errorf("unknown command %q (want serve, migrate, seed or sms-in)", cmd)
	}
	if err != nil {
		log.Error().Err(err).Msg("exiting")
		os.Exit(1)
	}
}

func newLogger(cfg *config.Config) zerolog.Logger {
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil || cfg.LogLevel == "" {
		level = zerolog.InfoLevel
	}
	var log zerolog.Logger
	if cfg.IsProduction() {
		log = zerolog.New(os.Stdout)
	} else {
		log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05"})
	}
	return log.Level(level).With().Timestamp().Logger()
}

// connect opens the database pool, waiting for the database to come up so
// the app and database can start together.
func connect(ctx context.Context, cfg *config.Config, log zerolog.Logger) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL: %w", err)
	}
	pc.MaxConns = 20
	pc.MaxConnIdleTime = 5 * time.Minute
	deadline := time.Now().Add(60 * time.Second)
	for {
		pool, err := pgxpool.NewWithConfig(ctx, pc)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return nil, fmt.Errorf("connect to database: %w", err)
		}
		log.Warn().Err(err).Msg("database not ready, retrying")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func withPool(ctx context.Context, cfg *config.Config, log zerolog.Logger, fn func(*pgxpool.Pool) error) error {
	pool, err := connect(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer pool.Close()
	return fn(pool)
}

// migrateLockKey is the advisory lock instances starting together take in
// turn, so that only the first applies the migrations. Arbitrary, and
// unlike the jobs' key.
const migrateLockKey = 7_311_043

func migrate(ctx context.Context, pool *pgxpool.Pool, log zerolog.Logger) error {
	lock, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	defer lock.Release()
	if _, err := lock.Exec(ctx, "SELECT pg_advisory_lock($1)", migrateLockKey); err != nil {
		return fmt.Errorf("migrate: wait for another instance: %w", err)
	}
	defer lock.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", migrateLockKey) //nolint:errcheck // the session's end frees it anyway

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	before, _ := goose.GetDBVersionContext(ctx, db)
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	after, _ := goose.GetDBVersionContext(ctx, db)
	if after != before {
		log.Info().Int64("from", before).Int64("to", after).Msg("database migrated")
	}
	return nil
}

// shutdownGrace is how long a stopping server gives requests in progress
// and the text or email being sent. Compose's stop_grace_period allows for it.
const shutdownGrace = 30 * time.Second

func serve(ctx context.Context, cfg *config.Config, log zerolog.Logger) error {
	pool, err := connect(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer pool.Close()
	if cfg.AutoMigrate {
		if err := migrate(ctx, pool, log); err != nil {
			return err
		}
	}

	a, err := app.New(cfg, pool, log, app.Options{})
	if err != nil {
		return err
	}
	svc := a.Services
	bg, stopBG := context.WithCancel(context.Background())
	defer stopBG()
	worked := make(chan struct{})
	go func() {
		a.Run(bg)
		close(worked)
	}()

	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           a.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// A sync with photos on a slow link: longer than the driver app's own
		// three minutes, so the app gives up first and knows to try again.
		ReadTimeout:    4 * time.Minute,
		WriteTimeout:   4 * time.Minute,
		IdleTimeout:    2 * time.Minute,
		MaxHeaderBytes: 1 << 16,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info().Str("addr", httpSrv.Addr).Str("url", cfg.PublicURL).Str("sms", svc.SMS.ProviderName()).
			Str("email", svc.Mail.ProviderName()).Str("payments", cfg.PaymentProvider).Msg(cfg.AppName + " is running")
		var printed []string
		if svc.SMS.ProviderName() == "mock" {
			printed = append(printed, "SMS")
		}
		if svc.Mail.ProviderName() == "mock" {
			printed = append(printed, "email")
		}
		if len(printed) > 0 {
			log.Info().Msgf("Stand-ins for %s: what they would send is printed here", strings.Join(printed, " and "))
		}
		if cfg.PaymentProvider == "mock" {
			log.Info().Msg("Payments are simulated: the providers' pages are pages of this server, and no money moves")
		}
		if cfg.SandboxURL != "" {
			log.Info().Str("sandbox", cfg.SandboxURL).Msg("The providers are the sandbox's: no text reaches a phone and no money moves")
		}
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	// Requests in progress finish first, so whatever they queue is in the
	// outbox. Then the workers finish the message in hand and hand back the
	// rest, which the next start sends.
	log.Info().Msg("shutting down")
	done, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	err = httpSrv.Shutdown(done)
	stopBG()
	select {
	case <-worked:
	case <-done.Done():
		log.Warn().Msg("stopped while still sending: what was being sent goes again within five minutes")
	}
	return err
}
