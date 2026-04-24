// Command notificationd starts the GOLD Notification Service.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/pkg/obs"

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
		v, err := jwtverify.NewVerifier(cfg.JWTPublicKeyFile)
		if err != nil {
			log.Warn("JWT verifier init failed — auth disabled", zap.Error(err))
		} else {
			verifyFunc = v.VerifyAccess
		}
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
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handlers.Routes(cfg.Env),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http listen", zap.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return nil
	}
}
