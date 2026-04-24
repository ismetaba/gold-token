package events

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	pkgevents "github.com/ismetaba/gold-token/backend/pkg/events"
	"github.com/ismetaba/gold-token/backend/services/notification/internal/channels"
	"github.com/ismetaba/gold-token/backend/services/notification/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/notification/internal/repo"
)

// Consumer handles notification-triggering events from the event bus.
type Consumer struct {
	bus        *pkgevents.Bus
	templates  repo.TemplateRepo
	deliveries repo.DeliveryRepo
	prefs      repo.PreferencesRepo
	userEmails repo.UserEmailRepo
	email      *channels.EmailSender
	webhook    *channels.WebhookSender
	log        *zap.Logger
	stream     string
}

func NewConsumer(
	bus *pkgevents.Bus,
	templates repo.TemplateRepo,
	deliveries repo.DeliveryRepo,
	prefs repo.PreferencesRepo,
	userEmails repo.UserEmailRepo,
	email *channels.EmailSender,
	webhook *channels.WebhookSender,
	log *zap.Logger,
	stream string,
) *Consumer {
	return &Consumer{
		bus: bus, templates: templates, deliveries: deliveries,
		prefs: prefs, userEmails: userEmails,
		email: email, webhook: webhook,
		log: log, stream: stream,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	subjects := []struct {
		durable string
		subject string
	}{
		{"notif_kyc_approved", pkgevents.SubjKYCApproved},
		{"notif_kyc_rejected", pkgevents.SubjKYCRejected},
		{"notif_order_created", pkgevents.SubjOrderCreated},
		{"notif_mint_executed", pkgevents.SubjMintExecuted},
		{"notif_burn_executed", pkgevents.SubjBurnExecuted},
		{"notif_compliance_alert", pkgevents.SubjComplianceAlert},
	}

	for _, s := range subjects {
		if err := c.bus.Subscribe(ctx, c.stream, s.durable, s.subject, c.handleEvent); err != nil {
			return err
		}
	}
	return nil
}

type genericPayload struct {
	UserID        string `json:"user_id"`
	ApplicationID string `json:"application_id"`
	OrderID       string `json:"order_id"`
}

func (c *Consumer) handleEvent(ctx context.Context, data []byte) error {
	var env pkgevents.Envelope[genericPayload]
	if err := json.Unmarshal(data, &env); err != nil {
		c.log.Warn("unmarshal notification event", zap.Error(err))
		return nil
	}

	userID, _ := uuid.Parse(env.Data.UserID)
	if userID == uuid.Nil {
		return nil // can't notify without user
	}

	// Look up template for this event type.
	tmpl, err := c.templates.ByEventType(ctx, env.EventType)
	if err != nil {
		// No template configured — skip silently.
		return nil
	}

	subject := renderTemplate(tmpl.SubjectTemplate, env)
	body := renderTemplate(tmpl.BodyTemplate, env)

	// Get user preferences.
	prefs, _ := c.prefs.ByUserID(ctx, userID)

	now := time.Now().UTC()

	for _, ch := range tmpl.Channels {
		switch ch {
		case "inapp":
			if !prefs.InappEnabled {
				continue
			}
			c.createDelivery(ctx, userID, tmpl.ID, ch, subject, body, "sent", "", now)

		case "email":
			if !prefs.EmailEnabled {
				continue
			}
			toAddr, lookupErr := c.resolveEmail(ctx, userID)
			if lookupErr != nil {
				c.log.Warn("resolve user email for notification", zap.Stringer("user_id", userID), zap.Error(lookupErr))
				c.createDelivery(ctx, userID, tmpl.ID, ch, subject, body, "failed", lookupErr.Error(), now)
				continue
			}
			sendErr := c.email.Send(ctx, toAddr, subject, body)
			status, errMsg := deliveryOutcome(sendErr)
			if sendErr != nil {
				c.log.Warn("email send failed", zap.String("to", toAddr), zap.Error(sendErr))
			}
			c.createDelivery(ctx, userID, tmpl.ID, ch, subject, body, status, errMsg, now)

		case "webhook":
			if !prefs.WebhookEnabled || prefs.WebhookURL == "" {
				continue
			}
			sendErr := c.webhook.Send(ctx, prefs.WebhookURL, env.EventType, subject, body)
			status, errMsg := deliveryOutcome(sendErr)
			if sendErr != nil {
				c.log.Warn("webhook send failed", zap.String("url", prefs.WebhookURL), zap.Error(sendErr))
			}
			c.createDelivery(ctx, userID, tmpl.ID, ch, subject, body, status, errMsg, now)
		}
	}

	return nil
}

func (c *Consumer) createDelivery(
	ctx context.Context,
	userID, templateID uuid.UUID,
	channel, subject, body, status, errMsg string,
	now time.Time,
) {
	sentAt := now
	d := domain.Delivery{
		ID:         uuid.Must(uuid.NewV7()),
		UserID:     userID,
		TemplateID: &templateID,
		Channel:    channel,
		Subject:    subject,
		Body:       body,
		Status:     status,
		Error:      errMsg,
		SentAt:     &sentAt,
		CreatedAt:  now,
	}
	if status == "failed" {
		d.SentAt = nil
	}
	if err := c.deliveries.Create(ctx, d); err != nil {
		c.log.Error("create delivery record", zap.String("channel", channel), zap.Error(err))
	}
}

// resolveEmail returns the user's email from the auth schema.
// Returns empty string in local mode when no userEmails repo is configured.
func (c *Consumer) resolveEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	if c.userEmails == nil {
		return "", nil // local mode: stub
	}
	email, err := c.userEmails.EmailByUserID(ctx, userID)
	if errors.Is(err, repo.ErrNotFound) {
		return "", nil // user not found — skip gracefully
	}
	return email, err
}

func deliveryOutcome(err error) (status, errMsg string) {
	if err == nil {
		return "sent", ""
	}
	return "failed", err.Error()
}

func renderTemplate(tmpl string, env pkgevents.Envelope[genericPayload]) string {
	r := strings.NewReplacer(
		"{{event_type}}", env.EventType,
		"{{user_id}}", env.Data.UserID,
		"{{order_id}}", env.Data.OrderID,
		"{{application_id}}", env.Data.ApplicationID,
	)
	return r.Replace(tmpl)
}
