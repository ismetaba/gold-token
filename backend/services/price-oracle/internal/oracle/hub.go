package oracle

import (
	"context"
	"sync"
)

// Hub manages WebSocket client subscriptions and broadcasts price updates.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}

	broadcast  chan []byte
	register   chan chan []byte
	unregister chan chan []byte
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[chan []byte]struct{}),
		broadcast:  make(chan []byte, 64),
		register:   make(chan chan []byte, 16),
		unregister: make(chan chan []byte, 16),
	}
}

// Subscribe registers a new client channel and returns an unsubscribe function.
func (h *Hub) Subscribe() (ch chan []byte, unsubscribe func()) {
	ch = make(chan []byte, 16)
	h.register <- ch
	return ch, func() { h.unregister <- ch }
}

// Broadcast queues a message to be sent to all subscribers.
func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
		// Drop message rather than block the oracle fetch loop.
	}
}

// Run processes register/unregister/broadcast events. Blocks until ctx is done.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ch := <-h.register:
			h.mu.Lock()
			h.clients[ch] = struct{}{}
			h.mu.Unlock()
		case ch := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[ch]; ok {
				delete(h.clients, ch)
				close(ch)
			}
			h.mu.Unlock()
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
