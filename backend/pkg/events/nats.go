package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// Bus is the high-level event bus wrapper. Wraps NATS JetStream.
type Bus struct {
	nc  *nats.Conn
	js  jetstream.JetStream
	log *zap.Logger
}

// NewBus connects and returns a ready Bus. Caller must Close().
func NewBus(url string, log *zap.Logger) (*Bus, error) {
	nc, err := nats.Connect(url,
		nats.Name("gold"),
		nats.Timeout(5*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	return &Bus{nc: nc, js: js, log: log}, nil
}

// Close tears down the connection.
func (b *Bus) Close() {
	if b.nc != nil {
		_ = b.nc.Drain()
	}
}

// Publish emits an event. Msg-Id = EventID for dedup.
func Publish[T any](ctx context.Context, b *Bus, env Envelope[T]) error {
	if env.EventID == uuid.Nil {
		env.EventID = uuid.Must(uuid.NewV7())
	}
	if env.OccurredAt.IsZero() {
		env.OccurredAt = time.Now().UTC()
	}
	if env.Version == 0 {
		env.Version = 1
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	msg := &nats.Msg{
		Subject: env.EventType,
		Data:    payload,
		Header:  nats.Header{},
	}
	msg.Header.Set(jetstream.MsgIDHeader, env.EventID.String())

	if _, err := b.js.PublishMsg(ctx, msg); err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	b.log.Debug("event published",
		zap.String("event_type", env.EventType),
		zap.String("event_id", env.EventID.String()),
		zap.String("aggregate_id", env.AggregateID),
	)
	return nil
}

// Subscribe creates a durable consumer for the given subject and handler.
// Handler errors cause the message to be NAK'd for retry.
func (b *Bus) Subscribe(
	ctx context.Context,
	stream, durable, subject string,
	handler func(ctx context.Context, data []byte) error,
) error {
	cons, err := b.js.CreateOrUpdateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Durable:       durable,
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,
	})
	if err != nil {
		return fmt.Errorf("consumer: %w", err)
	}

	_, err = cons.Consume(func(msg jetstream.Msg) {
		if err := handler(ctx, msg.Data()); err != nil {
			b.log.Warn("event handler failed",
				zap.String("subject", subject),
				zap.String("durable", durable),
				zap.Error(err),
			)
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	return err
}
