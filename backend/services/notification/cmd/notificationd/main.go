// Command notificationd starts the GOLD Notification Service.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/pkg/obs"
	"github.com/ismetaba/gold-token/backend/pkg/server"

	"github.com/ismetaba/gold-token/backend/services/notification/internal/channels"
	"github.com/ismetaba/gold-token/backend/services/notification/internal/config"
	notifevents "github.com/ismetaba/gold-token/backend/services/notification/internal/events"
	notifhttp "github.com/ismetaba/gold-token/backend/services/notification/internal/http"
	"github.com/ismetaba/gold-token/backend/services/notification/internal/jwtverify"
	"github.com/ismetaba/gold-token/backend/services/notification/internal/repo"
)

func main() {
	log := obs.NewLogger("notificationd")
	defer func() { _ = log.Sync() }()

	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal("config load failed", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log, cfg); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal("service exited with error", zap.Error(err))
	}
}

func run(ctx context.Context, log *zap.Logger, cfg *config.Config) error {
	var pool *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		var err error
		pool, err = pgxpool.New(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		log.Info("database connected")
	}

	var bus *pkgevents.Bus
	if cfg.NATSURL != "" {
		var err error
		bus, err = pkgevents.NewBus(cfg.NATSURL, log)
		if err != nil {
			return err
		}
		defer bus.Close()
		log.Info("NATS connected")
	}

	// JWT verifier for user auth.
	var verifyFunc func(string) (uuid.UUID, error)
	if cfg.JWTPublicKeyFile != "" {
		v, err := jwtverify.NewVerifier(cfg.JWTPublicKeyFile, cfg.Env)
		if err != nil {
			// Fail fast rather than silently running with auth disabled.
			return err
		}
		verifyFunc = v.VerifyAccess
	} else if cfg.Env != "local" {
		return errors.New("JWT_PUBLIC_KEY_FILE is required outside local env (refusing to run with auth disabled)")
	}

	var (
		deliveryRepo  repo.DeliveryRepo
		templateRepo  repo.TemplateRepo
		prefsRepo     repo.PreferencesRepo
		userEmailRepo repo.UserEmailRepo
	)
	if pool != nil {
		deliveryRepo = repo.NewPGDeliveryRepo(pool)
		templateRepo = repo.NewPGTemplateRepo(pool)
		prefsRepo = repo.NewPGPreferencesRepo(pool)
		userEmailRepo = repo.NewPGUserEmailRepo(pool)
	}

	emailSender := channels.NewEmailSender(
		cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom, cfg.SendGridAPIKey,
	)
	webhookSender := channels.NewWebhookSender()

	if bus != nil && deliveryRepo != nil {
		cons := notifevents.NewConsumer(
			bus, templateRepo, deliveryRepo, prefsRepo,
			userEmailRepo, emailSender, webhookSender,
			log, cfg.NATSStream,
		)
		if err := cons.Start(ctx); err != nil {
			return err
		}
		log.Info("notification event consumer started")
	}

	handlers := notifhttp.NewHandlers(deliveryRepo, prefsRepo, verifyFunc, log)
	srv := server.NewHTTPServer(cfg.HTTPAddr, handlers.Routes(cfg.Env), server.DefaultTimeouts())
	return server.Serve(ctx, srv, log, 10*time.Second)
}
