package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"mirror/server/internal/live"
	"mirror/server/internal/swipes"
	"mirror/server/internal/users"
)

const (
	pingEvery    = 30 * time.Second
	writeTimeout = 10 * time.Second
)

// GET /v1/ws: one WebSocket per open tab. The server pushes live events
// (matches, messages, read marks, typing); the browser sends only "typing".
// Messages themselves are sent with POST, which can be retried safely.
func (s *server) liveSocket(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	// Accept also checks that the page's origin is this server's host.
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept has already answered the request
	}
	defer conn.CloseNow()
	conn.SetReadLimit(4096)

	events, stop := s.hub.Subscribe(user.ID)
	defer stop()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		defer cancel()
		s.readTyping(ctx, conn, user)
	}()

	ping := time.NewTicker(pingEvery)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case e, open := <-events:
			if !open {
				conn.Close(websocket.StatusTryAgainLater, "too far behind; reconnect")
				return
			}
			data, _ := json.Marshal(e)
			wctx, done := context.WithTimeout(ctx, writeTimeout)
			err := conn.Write(wctx, websocket.MessageText, data)
			done()
			if err != nil {
				return
			}
		case <-ping.C:
			pctx, done := context.WithTimeout(ctx, writeTimeout)
			err := conn.Ping(pctx)
			done()
			if err != nil {
				return
			}
		}
	}
}

// readTyping passes "typing" from the browser on to the other person in the
// match, until the connection ends.
func (s *server) readTyping(ctx context.Context, conn *websocket.Conn, user users.User) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var in struct {
			Type    string `json:"type"`
			MatchID string `json:"match_id"`
		}
		if json.Unmarshal(data, &in) != nil || in.Type != live.TypeTyping {
			continue
		}
		m, err := swipes.Get(ctx, s.db, user, in.MatchID)
		if err != nil {
			continue
		}
		s.hub.Send(m.OtherUserID(user.ID), live.Event{Type: live.TypeTyping, MatchID: m.ID})
	}
}
