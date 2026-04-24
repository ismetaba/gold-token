package domain

import (
	"time"

	"github.com/google/uuid"
)

type Delivery struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TemplateID *uuid.UUID
	Channel    string // "inapp", "email", "webhook"
	Subject    string
	Body       string
	Status     string // "pending", "sent", "failed", "read"
	Error      string
	SentAt     *time.Time
	CreatedAt  time.Time
}

type Template struct {
	ID              uuid.UUID
	EventType       string
	SubjectTemplate string
	BodyTemplate    string
	Channels        []string
	Active          bool
}

type Preferences struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	EmailEnabled   bool
	WebhookURL     string
	WebhookEnabled bool
	InappEnabled   bool
	UpdatedAt      time.Time
}
