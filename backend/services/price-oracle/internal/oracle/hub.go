package oracle

import (
	"context"
	"sync"
)

// Hub manages WebSocket client subscriptions and broadcasts price updates.
//
// Registration/unregistration are performed directly under the mutex rather
// than via channels, so a Subscribe/unsubscribe that races with shutdown can
// never block forever waiting for Run to drain a channel it has already
// stopped reading.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
	closed  bool

	broadcast chan []byte
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:   make(map[chan []byte]struct{}),
		broadcast: make(chan []byte, 64),
	}
}

// Subscribe registers a new client channel and returns an unsubscribe function.
// If the hub has already shut down, it returns an already-closed channel and a
// no-op unsubscribe so callers never block.
func (h *Hub) Subscribe() (ch chan []byte, unsubscribe func()) {
	ch = make(chan []byte, 16)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(ch)
		return ch, func() {}
	}
	h.clients[ch] = struct{}{}
	return ch, func() { h.removeClient(ch) }
}

func (h *Hub) removeClient(ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
}

// Broadcast queues a message to be sent to all subscribers.
func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
		// Drop message rather than block the oracle fetch loop.
	}
}

// Run processes broadcast events. Blocks until ctx is done, at which point all
// client channels are closed.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			h.closed = true
			for ch := range h.clients {
				delete(h.clients, ch)
				close(ch)
			}
			h.mu.Unlock()
			return
		case msg := <-h.broadcast:
			h.mu.RLock()
			for ch := range h.clients {
				select {
				case ch <- msg:
				default:
					// Slow client — drop rather than block.
				}
			}
			h.mu.RUnlock()
		}
	}
}
