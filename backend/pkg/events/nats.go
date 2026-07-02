package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// maxDeliver is the number of delivery attempts before JetStream stops
// redelivering a message. The last attempt is treated as terminal.
const maxDeliver = 5

// permanentError marks a handler error as non-retryable (e.g. a malformed
// payload or a validation failure). Such messages are terminated immediately
// rather than retried until MaxDeliver and then silently dropped.
type permanentError struct{ err error }

func (e permanentError) Error() string { return "permanent: " + e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// Permanent wraps err to signal that redelivery is pointless. Consumers should
// wrap validation/decoding failures with it so poison messages are not retried.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err: err}
}

// IsPermanent reports whether err (or anything it wraps) is a permanent error.
func IsPermanent(err error) bool {
	var pe permanentError
	return errors.As(err, &pe)
}

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

// BuildEnvelope fills in the default envelope fields (EventID, OccurredAt,
// Version) and marshals it to JSON, returning the event ID and bytes. It is
// used to stage an event in a transactional outbox so the event is persisted
// atomically with the state change that produced it, then published later by a
// relay (see PublishRaw).
func BuildEnvelope[T any](env Envelope[T]) (uuid.UUID, []byte, error) {
	if env.EventID == uuid.Nil {
		env.EventID = uuid.Must(uuid.NewV7())
	}
	if env.OccurredAt.IsZero() {
		env.OccurredAt = time.Now().UTC()
	}
	if env.Version == 0 {
		env.Version = 1
	}
	b, err := json.Marshal(env)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("marshal envelope: %w", err)
	}
	return env.EventID, b, nil
}

// PublishRaw publishes a pre-marshalled payload to subject. msgID is set as the
// JetStream dedup key (Msg-Id), so re-publishing the same outbox row is
// idempotent.
func (b *Bus) PublishRaw(ctx context.Context, subject, msgID string, payload []byte) error {
	msg := &nats.Msg{Subject: subject, Data: payload, Header: nats.Header{}}
	if msgID != "" {
		msg.Header.Set(jetstream.MsgIDHeader, msgID)
	}
	if _, err := b.js.PublishMsg(ctx, msg); err != nil {
		return fmt.Errorf("publish raw: %w", err)
	}
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
		MaxDeliver:    maxDeliver,
	})
	if err != nil {
		return fmt.Errorf("consumer: %w", err)
	}

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		err := handler(ctx, msg.Data())
		if err == nil {
			_ = msg.Ack()
			return
		}

		// Decide between retry (transient) and terminate (poison). A message is
		// terminated when the error is explicitly permanent, or when this was
		// the final delivery attempt — otherwise JetStream would silently drop
		// it after MaxDeliver with no record.
		var delivered uint64
		if meta, merr := msg.Metadata(); merr == nil {
			delivered = meta.NumDelivered
		}
		permanent := IsPermanent(err)
		if permanent || delivered >= maxDeliver {
			b.log.Error("event handler gave up; terminating message (poison)",
				zap.String("subject", subject),
				zap.String("durable", durable),
				zap.Uint64("delivered", delivered),
				zap.Bool("permanent", permanent),
				zap.Error(err),
			)
			_ = msg.Term()
			return
		}

		b.log.Warn("event handler failed; will retry",
			zap.String("subject", subject),
			zap.String("durable", durable),
			zap.Uint64("delivered", delivered),
			zap.Error(err),
		)
		_ = msg.Nak()
	}, jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, cErr error) {
		// Surface async consumer errors (heartbeat loss, pull failures, ...) that would
		// otherwise be silently swallowed and hide a stalled consumer.
		b.log.Error("jetstream consume error",
			zap.String("subject", subject),
			zap.String("durable", durable),
			zap.Error(cErr),
		)
	}))
	if err != nil {
		return err
	}

	// Stop consuming when the parent context is cancelled so the consumer shuts down
	// cleanly on service exit instead of leaking.
	go func() {
		<-ctx.Done()
		cc.Stop()
	}()
	return nil
}
