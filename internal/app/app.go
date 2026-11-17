// Package app assembles Kiflet from its settings: the providers, the business
// services, the web server and the background workers. The server command
// and the integration tests both build it here, so the tests run the wiring
// that production runs.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/natnael-alemayehu/kiflet/internal/config"
	"github.com/natnael-alemayehu/kiflet/internal/service"
	"github.com/natnael-alemayehu/kiflet/internal/web"
	"github.com/natnael-alemayehu/kiflet/pkg/console"
	"github.com/natnael-alemayehu/kiflet/pkg/mail"
	"github.com/natnael-alemayehu/kiflet/pkg/payments"
	"github.com/natnael-alemayehu/kiflet/pkg/sms"
	"github.com/natnael-alemayehu/kiflet/pkg/storage"
	"github.com/natnael-alemayehu/kiflet/pkg/websocket"
)

// Options change how the app is put together. The zero value is
// production's.
type Options struct {
	// Console is where the stand-in providers print what they would have
	// sent. Nil prints to standard output.
	Console *console.Console
	// NoJobs leaves the periodic jobs out of Run. The integration tests
	// share one database and run a job only when a test calls for it.
	NoJobs bool
}

// App is Kiflet, assembled.
type App struct {
	Services *service.Services
	web      *web.Server
	live     *liveRelay
	noJobs   bool
}

// New builds the app. It refuses stand-in providers in production unless
// the settings allow them.
func New(cfg *config.Config, pool *pgxpool.Pool, log zerolog.Logger, opts Options) (*App, error) {
	if mocked := cfg.MockedProviders(); cfg.IsProduction() && len(mocked) > 0 && !cfg.AllowMockProviders {
		return nil, fmt.Errorf("%s would be stand-ins in production, sending nothing and taking no money: configure real providers, "+
			"or set ALLOW_MOCK_PROVIDERS=true for a demo site", strings.Join(mocked, ", "))
	}
	if cfg.IsProduction() && cfg.SandboxURL != "" && !cfg.AllowMockProviders {
		return nil, errors.New("SANDBOX_URL is set, so the providers are the sandbox, which sends no texts and takes no money: " +
			"unset it in production, or set ALLOW_MOCK_PROVIDERS=true for a demo site")
	}
	out := opts.Console
	if out == nil {
		out = console.New(os.Stdout)
	}
	providers, err := paymentProviders(cfg)
	if err != nil {
		return nil, fmt.Errorf("payments: %w", err)
	}
	store, err := storage.New(&storage.Config{
		Type: cfg.StorageType, LocalPath: cfg.StoragePath,
		S3Bucket: cfg.S3Bucket, S3Region: cfg.S3Region, S3Endpoint: cfg.S3Endpoint,
		S3AccessKey: cfg.S3AccessKey, S3SecretKey: cfg.S3SecretKey, S3UseSSL: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	// Opt-outs and pacing live in the service's outbox.
	provider, err := sms.NewProvider(sms.Config{
		Provider:           sms.ProviderType(cfg.SMSProvider),
		EthioTelecomAPIURL: cfg.EthioTelecomAPIURL, EthioTelecomAPIKey: cfg.EthioTelecomAPIKey, EthioTelecomSenderID: cfg.EthioTelecomSenderID,
		Console: out,
	}, smsLog{log.With().Str("component", "sms").Logger()})
	if err != nil {
		return nil, fmt.Errorf("sms: %w", err)
	}
	mailer, err := mail.New(mail.Config{
		Provider: cfg.MailProvider, Console: out,
		SMTP: mail.SMTP{Addr: cfg.SMTPAddr, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, From: cfg.MailFrom},
	})
	if err != nil {
		return nil, fmt.Errorf("email: %w", err)
	}

	hub := websocket.NewHub(log.With().Str("component", "live").Logger())
	go hub.Run()
	// Updates reach the hub through Postgres, so that every instance's
	// dashboards get them.
	live := &liveRelay{pool: pool, hub: hub, log: log.With().Str("component", "live").Logger()}
	svc := service.New(service.Deps{
		Pool: pool, Log: log, Publisher: live, Storage: store, SMS: smsSender{provider}, Mail: mailSender{mailer},
		Payments: providers,
		Settings: service.Settings{
			AppName: cfg.AppName, BaseURL: cfg.PublicURL, Location: cfg.Timezone,
			Secret: cfg.Secret, SMSNumber: cfg.SMSNumber,
			SupportEmail: cfg.SupportEmail, BankDetails: cfg.BankDetails,
			RetainShipments: time.Duration(cfg.RetainShipmentDays) * 24 * time.Hour,
			CodesPerDay:     cfg.CodesPerDay,
			PrivateWebhooks: cfg.PrivateWebhooks,
		},
	})
	srv, err := web.New(cfg, svc, pool, hub, log)
	if err != nil {
		return nil, err
	}
	return &App{Services: svc, web: srv, live: live, noJobs: opts.NoJobs}, nil
}

// Handler serves the dashboard, the public pages, the driver app and the
// APIs.
func (a *App) Handler() http.Handler { return a.web.Handler() }

// Run does the background work until ctx ends: passing live updates
// between instances, sending texts, email and webhooks, and the periodic
// jobs. It returns once each worker has finished what it had in
// hand. Texts and email it had taken but not started are handed back, for
// the next start to send at once.
func (a *App) Run(ctx context.Context) {
	workers := []func(context.Context){a.live.Run, a.Services.SMS.Run, a.Services.Mail.Run, a.Services.Webhooks.Run}
	if !a.noJobs {
		workers = append(workers, a.Services.Jobs.Run)
	}
	var wg sync.WaitGroup
	for _, run := range workers {
		wg.Go(func() { run(ctx) })
	}
	wg.Wait()
}

// paymentProviders are the mobile-money methods offered. The stand-ins are
// a telebirr whose checkout is a page on this server, and the real CBE Birr
// adapter, which only answers calls, called by a page playing CBE Birr.
func paymentProviders(cfg *config.Config) ([]payments.Provider, error) {
	cbe := payments.CBEBirrConfig{
		MerchantID: cfg.CBEBirr.MerchantID, Username: cfg.CBEBirr.CallbackUsername, Password: cfg.CBEBirr.CallbackPassword,
	}
	switch cfg.PaymentProvider {
	case "none":
		return nil, nil
	case "mock":
		checkout := func(reference string) string { return cfg.PublicURL + "/dev/payments/" + reference }
		cbe.StandIn = true
		standIn, err := payments.NewCBEBirr(cbe)
		if err != nil {
			return nil, fmt.Errorf("cbebirr: %w", err)
		}
		return []payments.Provider{payments.NewMock("telebirr", "telebirr", cfg.Secret, checkout), standIn}, nil
	}
	var out []payments.Provider
	for _, code := range cfg.PaymentProviders() {
		var p payments.Provider
		var err error
		switch code {
		case "telebirr":
			p, err = telebirr(cfg.Telebirr)
		case "cbebirr":
			p, err = payments.NewCBEBirr(cbe)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", code, err)
		}
		out = append(out, p)
	}
	return out, nil
}

func telebirr(t config.TelebirrSettings) (*payments.Telebirr, error) {
	key, err := os.ReadFile(t.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("TELEBIRR_PRIVATE_KEY_FILE: %w", err)
	}
	pub, err := os.ReadFile(t.PublicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("TELEBIRR_PUBLIC_KEY_FILE: %w", err)
	}
	return payments.NewTelebirr(payments.TelebirrConfig{
		APIURL: t.APIURL, CheckoutURL: t.CheckoutURL, FabricAppID: t.FabricAppID, AppSecret: t.AppSecret,
		MerchantAppID: t.MerchantAppID, MerchantCode: t.MerchantCode, PrivateKey: key, PublicKey: pub,
	})
}

// smsSender adapts the provider package to the service's outbox.
type smsSender struct{ p sms.SMSProvider }

func (s smsSender) Name() string { return s.p.Name() }
func (s smsSender) Send(ctx context.Context, to, body string) (string, error) {
	id, err := s.p.Send(ctx, to, body, sms.MessageTypeNotification)
	if errors.Is(err, sms.ErrRejected) {
		err = fmt.Errorf("%w: %w", service.ErrSMSRejected, err)
	}
	return id, err
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
