// Package live pushes events to the open WebSocket connections of a user.
// The connections are kept in memory, so this works for one server; more
// servers would need a shared channel (such as Redis pub/sub) behind Send.
package live

import (
	"sync"
	"time"
)

// Event types sent to the browser.
const (
	TypeMatch          = "match"           // a new match
	TypeUnmatched      = "unmatched"       // a match was ended
	TypeMessage        = "message"         // a new message
	TypeMessageDeleted = "message_deleted" // a message was deleted
	TypeRead           = "read"            // the other person read the chat
	TypeTyping         = "typing"          // the other person is typing
)

// Event is one push to the browser.
type Event struct {
	Type      string     `json:"type"`
	MatchID   string     `json:"match_id"`
	Message   any        `json:"message,omitempty"`
	MessageID string     `json:"message_id,omitempty"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
}

// queueSize is how many events a connection may fall behind by. A
// connection further behind is closed; the browser reconnects and reloads.
const queueSize = 32

type Hub struct {
	mu    sync.Mutex
	conns map[string]map[chan Event]bool
}

func NewHub() *Hub {
	return &Hub{conns: map[string]map[chan Event]bool{}}
}

// Subscribe registers one connection of a user. The channel is closed when
// the connection falls too far behind. Call stop when the connection ends.
func (h *Hub) Subscribe(userID string) (events <-chan Event, stop func()) {
	ch := make(chan Event, queueSize)
	h.mu.Lock()
	if h.conns[userID] == nil {
		h.conns[userID] = map[chan Event]bool{}
	}
	h.conns[userID][ch] = true
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.remove(userID, ch)
	}
}

// Send queues the event for every open connection of the user, and reports
// whether the user had one.
func (h *Hub) Send(userID string, e Event) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	sent := false
	for ch := range h.conns[userID] {
		select {
		case ch <- e:
			sent = true
		default:
			h.remove(userID, ch)
		}
	}
	return sent
}

// remove closes a connection's channel once; h.mu must be held.
func (h *Hub) remove(userID string, ch chan Event) {
	if !h.conns[userID][ch] {
		return
	}
	delete(h.conns[userID], ch)
	if len(h.conns[userID]) == 0 {
		delete(h.conns, userID)
	}
	close(ch)
}
