package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/repo"
)

type fakeOutboxRepo struct {
	mu        sync.Mutex
	rows      []repo.OutboxRow
	published map[uuid.UUID]bool
}

func (f *fakeOutboxRepo) FetchUnpublished(_ context.Context, limit int) ([]repo.OutboxRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []repo.OutboxRow
	for _, r := range f.rows {
		if !f.published[r.ID] {
			out = append(out, r)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeOutboxRepo) MarkPublished(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published[id] = true
	return nil
}

type fakePublisher struct {
	mu       sync.Mutex
	calls    []string // msgIDs
	failNext bool
}

func (p *fakePublisher) PublishRaw(_ context.Context, _, msgID string, _ []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failNext {
		return errors.New("publish boom")
	}
	p.calls = append(p.calls, msgID)
	return nil
}

func newRelayHarness() (*Relay, *fakeOutboxRepo, *fakePublisher) {
	id1, id2 := uuid.New(), uuid.New()
	r := &fakeOutboxRepo{
		rows: []repo.OutboxRow{
			{ID: id1, Subject: "gold.mint.executed.v1", Payload: []byte(`{}`)},
			{ID: id2, Subject: "gold.mint.failed.v1", Payload: []byte(`{}`)},
		},
		published: map[uuid.UUID]bool{},
	}
	p := &fakePublisher{}
	return NewRelay(r, p, zap.NewNop(), 0), r, p
}

func TestRelayDrainPublishesAndMarks(t *testing.T) {
	relay, r, p := newRelayHarness()
	if err := relay.drain(context.Background()); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if len(p.calls) != 2 {
		t.Fatalf("published %d events, want 2", len(p.calls))
	}
	// All rows marked published => a second drain publishes nothing.
	p.calls = nil
	if err := relay.drain(context.Background()); err != nil {
		t.Fatalf("drain 2: %v", err)
	}
	if len(p.calls) != 0 {
		t.Fatalf("re-published %d already-acked events", len(p.calls))
	}
	_ = r
}

func TestRelayLeavesRowUnpublishedOnPublishError(t *testing.T) {
	relay, r, p := newRelayHarness()
	p.failNext = true
	if err := relay.drain(context.Background()); err != nil {
		t.Fatalf("drain should swallow publish errors for retry, got %v", err)
	}
	// Nothing should be marked published; a later successful drain delivers both.
	p.failNext = false
	if err := relay.drain(context.Background()); err != nil {
		t.Fatalf("drain retry: %v", err)
	}
	if len(p.calls) != 2 {
		t.Fatalf("after recovery published %d, want 2", len(p.calls))
	}
	_ = r
}

func TestRelayUsesRowIDAsMsgID(t *testing.T) {
	relay, r, p := newRelayHarness()
	_ = relay.drain(context.Background())
	want := map[string]bool{r.rows[0].ID.String(): true, r.rows[1].ID.String(): true}
	for _, id := range p.calls {
		if !want[id] {
			t.Fatalf("unexpected msgID %q (must be the outbox row ID for dedup)", id)
		}
	}
}
