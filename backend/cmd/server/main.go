// Command server runs Menged: the dispatcher dashboard, public tracking, the
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

	"github.com/natnael-alemayehu/logistics/internal/config"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/internal/web"
	"github.com/natnael-alemayehu/logistics/migrations"
	"github.com/natnael-alemayehu/logistics/pkg/console"
	"github.com/natnael-alemayehu/logistics/pkg/mail"
	"github.com/natnael-alemayehu/logistics/pkg/payments"
	"github.com/natnael-alemayehu/logistics/pkg/sms"
	"github.com/natnael-alemayehu/logistics/pkg/storage"
	"github.com/natnael-alemayehu/logistics/pkg/websocket"
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

func migrate(ctx context.Context, pool *pgxpool.Pool, log zerolog.Logger) error {
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

// smsSender adapts the provider package to the service's outbox.
type smsSender struct{ p sms.SMSProvider }

func (s smsSender) Name() string { return s.p.Name() }
func (s smsSender) Send(ctx context.Context, to, body string) error {
	return s.p.Send(ctx, to, body, sms.MessageTypeNotification)
}

// mailSender adapts the mail package to the service's outbox.
type mailSender struct{ s mail.Sender }

func (m mailSender) Name() string { return m.s.Name() }
func (m mailSender) Send(ctx context.Context, to, subject, body string) error {
	return m.s.Send(ctx, mail.Message{To: to, Subject: subject, Text: body})
}

// smsLog routes the provider's logging into ours.
type smsLog struct{ l zerolog.Logger }

func (s smsLog) Info(msg string, kv ...interface{})  { s.l.Info().Fields(kv).Msg(msg) }
func (s smsLog) Error(msg string, kv ...interface{}) { s.l.Error().Fields(kv).Msg(msg) }
func (s smsLog) Warn(msg string, kv ...interface{})  { s.l.Warn().Fields(kv).Msg(msg) }
func (s smsLog) Debug(msg string, kv ...interface{}) { s.l.Debug().Fields(kv).Msg(msg) }

// terminal is where the stand-in providers print what they would have sent.
var terminal = console.New(os.Stdout)

// paymentProviders are the mobile-money methods offered. There are no real
// adapters yet: the stand-ins print each payment request with a link to a
// page that approves or declines it.
func paymentProviders(cfg *config.Config) []payments.Provider {
	if cfg.PaymentProvider != "mock" {
		return nil
	}
	prompt := func(reference string) string { return cfg.PublicURL + "/dev/payments/" + reference }
	return []payments.Provider{
		payments.NewMock("telebirr", "telebirr", cfg.Secret, prompt, terminal),
		payments.NewMock("cbebirr", "CBE Birr", cfg.Secret, prompt, terminal),
	}
}

func buildServices(cfg *config.Config, pool *pgxpool.Pool, pub service.Publisher, log zerolog.Logger) (*service.Services, error) {
	if mocked := cfg.MockedProviders(); cfg.IsProduction() && len(mocked) > 0 && !cfg.AllowMockProviders {
		return nil, fmt.Errorf("%s would only be printed to the terminal in production: configure real providers, "+
			"or set ALLOW_MOCK_PROVIDERS=true for a demo site", strings.Join(mocked, ", "))
	}
	store, err := storage.New(&storage.Config{
		Type: cfg.StorageType, LocalPath: cfg.StoragePath,
		S3Bucket: cfg.S3Bucket, S3Region: cfg.S3Region, S3Endpoint: cfg.S3Endpoint,
		S3AccessKey: cfg.S3AccessKey, S3SecretKey: cfg.S3SecretKey, S3UseSSL: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	// Opt-outs and pacing live in the service's outbox, so the provider gets
	// neither store.
	provider, err := sms.NewProvider(sms.Config{
		Provider:         sms.ProviderType(cfg.SMSProvider),
		TwilioAccountSID: cfg.TwilioAccountSID, TwilioAuthToken: cfg.TwilioAuthToken, TwilioFromNumber: cfg.TwilioFromNumber,
		EthioTelecomAPIURL: cfg.EthioTelecomAPIURL, EthioTelecomAPIKey: cfg.EthioTelecomAPIKey, EthioTelecomSenderID: cfg.EthioTelecomSenderID,
		Console: terminal,
	}, smsLog{log.With().Str("component", "sms").Logger()}, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("sms: %w", err)
	}
	mailer, err := mail.New(mail.Config{
		Provider: cfg.MailProvider, Console: terminal,
		SMTP: mail.SMTP{Addr: cfg.SMTPAddr, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, From: cfg.MailFrom},
	})
	if err != nil {
		return nil, fmt.Errorf("email: %w", err)
	}
	return service.New(service.Deps{
		Pool: pool, Log: log, Publisher: pub, Storage: store, SMS: smsSender{provider}, Mail: mailSender{mailer},
		Payments: paymentProviders(cfg),
		Settings: service.Settings{
			AppName: cfg.AppName, BaseURL: cfg.PublicURL, Location: cfg.Timezone,
			Secret: cfg.Secret, SMSNumber: cfg.SMSNumber,
			SupportEmail: cfg.SupportEmail, BankDetails: cfg.BankDetails,
		},
	}), nil
}

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

	hub := websocket.NewHub(log.With().Str("component", "live").Logger())
	go hub.Run()

	svc, err := buildServices(cfg, pool, hub, log)
	if err != nil {
		return err
	}
	srv, err := web.New(cfg, svc, pool, hub, log)
	if err != nil {
		return err
	}

	bg, stopBG := context.WithCancel(context.Background())
	defer stopBG()
	go svc.SMS.Run(bg)
	go svc.Mail.Run(bg)
	go svc.Jobs.Run(bg)

	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute, // a sync with photos on a slow link
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 16,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info().Str("addr", httpSrv.Addr).Str("url", cfg.PublicURL).Str("sms", svc.SMS.ProviderName()).
			Str("email", svc.Mail.ProviderName()).Str("payments", cfg.PaymentProvider).Msg(cfg.AppName + " is running")
		if mocked := cfg.MockedProviders(); len(mocked) > 0 {
			log.Info().Msgf("Stand-ins for %s: what they would send is printed here", strings.Join(mocked, ", "))
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
	log.Info().Msg("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err = httpSrv.Shutdown(shutdownCtx)
	stopBG()
	return err
}
