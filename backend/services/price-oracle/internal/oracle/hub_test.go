package oracle

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestHubBroadcastReachesSubscriber(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	ch, unsub := h.Subscribe()
	defer unsub()

	// Give Run a moment to register (registration is synchronous under the
	// mutex, so this is just scheduling slack).
	time.Sleep(5 * time.Millisecond)
	h.Broadcast([]byte("hello"))

	select {
	case msg := <-ch:
		if string(msg) != "hello" {
			t.Fatalf("got %q want hello", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive broadcast")
	}
}

func TestHubUnsubscribeClosesChannel(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	ch, unsub := h.Subscribe()
	unsub()
	// Reading a closed channel must not block.
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("channel should be closed after unsubscribe")
		}
	case <-time.After(time.Second):
		t.Fatal("read on unsubscribed channel blocked")
	}
}

// TestHubSubscribeDoesNotDeadlockDuringShutdown is the regression test for the
// original bug: Subscribe blocked forever sending to a register channel that
// Run had stopped draining. Subscribing concurrently with shutdown must never
// hang.
func TestHubSubscribeDoesNotDeadlockDuringShutdown(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)

	done := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, unsub := h.Subscribe()
			_ = ch
			unsub()
		}()
	}
	cancel() // shut down while subscribers are racing

	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Subscribe/unsubscribe deadlocked during shutdown")
	}
}

func TestHubSubscribeAfterShutdownReturnsClosedChannel(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	cancel()
	time.Sleep(10 * time.Millisecond) // let Run observe cancellation

	ch, unsub := h.Subscribe()
	defer unsub()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected an already-closed channel after shutdown")
		}
	case <-time.After(time.Second):
		t.Fatal("Subscribe after shutdown returned an open channel")
	}
}
