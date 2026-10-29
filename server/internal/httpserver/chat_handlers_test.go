package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// wsEvent is a live event as the browser reads it.
type wsEvent struct {
	Type      string           `json:"type"`
	MatchID   string           `json:"match_id"`
	MessageID string           `json:"message_id"`
	ReadAt    *time.Time       `json:"read_at"`
	Message   *MessageResponse `json:"message"`
}

// socket opens the live WebSocket as this client, from a page on origin.
func (c *authClient) socket(origin string) (*websocket.Conn, *http.Response, error) {
	h := http.Header{}
	if origin != "" {
		h.Set("Origin", origin)
	}
	url := "ws" + strings.TrimPrefix(c.base, "http") + "/v1/ws"
	conn, res, err := websocket.Dial(context.Background(), url, &websocket.DialOptions{HTTPClient: c.http, HTTPHeader: h})
	if conn != nil {
		c.t.Cleanup(func() { conn.CloseNow() })
	}
	return conn, res, err
}

// waitEvent reads events until one of type typ arrives.
func waitEvent(t *testing.T, conn *websocket.Conn, typ string) wsEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for a %q event: %v", typ, err)
		}
		var e wsEvent
		json.Unmarshal(data, &e)
		if e.Type == typ {
			return e
		}
	}
}

// sendFile posts a message with an attachment.
func (c *authClient) sendFile(matchID, body, clientID, name string, data []byte, out any) *http.Response {
	c.t.Helper()
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	form.WriteField("body", body)
	form.WriteField("client_id", clientID)
	part, _ := form.CreateFormFile("file", name)
	part.Write(data)
	form.Close()
	res, err := c.http.Post(c.base+"/v1/matches/"+matchID+"/messages", form.FormDataContentType(), &buf)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil {
		json.NewDecoder(res.Body).Decode(out)
	}
	return res
}

// The done check for chunk 10: a match alert arrives live, and both sides
// can message.
func TestChatBetweenAMatchedPair(t *testing.T) {
	employer := newAuthClient(t)
	employer.signInAs("chat.hirer@example.com", "employer")
	employer.do("PUT", "/v1/me", map[string]string{"first_name": "Ana"}, nil)
	var card HiringCardResponse
	employer.do("POST", "/v1/hiring-cards", map[string]string{"description": jobDescription}, &card)
	employer.do("PUT", "/v1/hiring-cards/"+card.ID+"/status", map[string]string{"status": "live"}, nil)
	candidate := *employer
	candidate.http = &http.Client{Jar: newJar()}
	candidateID := finishedCandidate(t, &candidate, "chat.seeker@example.com")
	candidate.swipe(map[string]string{"hiring_card_id": card.ID, "direction": "right"}, "c1", nil)

	// The socket needs a session, and a page from this server.
	nobody := *employer
	nobody.http = &http.Client{Jar: newJar()}
	if _, res, err := nobody.socket(""); err == nil || res.StatusCode != http.StatusUnauthorized {
		t.Errorf("socket without a session: %v, want 401", err)
	}
	if _, res, err := candidate.socket("https://evil.example"); err == nil || res.StatusCode != http.StatusForbidden {
		t.Errorf("socket from another site: %v, want 403", err)
	}
	candSock, _, err := candidate.socket(candidate.base)
	if err != nil {
		t.Fatal(err)
	}

	// The employer likes back: the candidate gets the alert live.
	var made SwipeResponse
	employer.swipe(map[string]string{"hiring_card_id": card.ID, "candidate_id": candidateID, "direction": "right"}, "e1", &made)
	if e := waitEvent(t, candSock, "match"); e.MatchID != made.MatchID {
		t.Errorf("match alert for %q, want %q", e.MatchID, made.MatchID)
	}
	var logged int64
	employer.tx.Raw("SELECT count(*) FROM events.notifications WHERE user_id = ? AND kind = 'match' AND delivered", candidateID).Scan(&logged)
	if logged != 1 {
		t.Errorf("%d delivered match notifications logged, want 1", logged)
	}
	empSock, _, err := employer.socket(employer.base)
	if err != nil {
		t.Fatal(err)
	}
	var counts UnreadResponse
	if candidate.do("GET", "/v1/unread", nil, &counts); counts != (UnreadResponse{NewMatches: 1}) {
		t.Errorf("candidate's unread = %+v, want 1 new match", counts)
	}
	chatPath := "/v1/matches/" + made.MatchID

	// Typing, then a text message from the employer.
	candSock.Write(context.Background(), websocket.MessageText, []byte(`{"type":"typing","match_id":"`+made.MatchID+`"}`))
	waitEvent(t, empSock, "typing")

	if res := employer.do("POST", chatPath+"/messages", map[string]string{"body": " ", "client_id": "m1"}, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("empty message: status %d, want 400", res.StatusCode)
	}
	var hello, retry MessageResponse
	employer.do("POST", chatPath+"/messages", map[string]string{"body": "Hello Jordan", "client_id": "m1"}, &hello)
	employer.do("POST", chatPath+"/messages", map[string]string{"body": "Hello Jordan", "client_id": "m1"}, &retry)
	if hello.ID == "" || retry.ID != hello.ID {
		t.Errorf("message %+v, retry %+v; want one message", hello, retry)
	}
	if e := waitEvent(t, candSock, "message"); e.Message == nil || e.Message.Body != "Hello Jordan" {
		t.Errorf("candidate's live message = %+v", e.Message)
	}

	// The candidate answers with a file.
	var reply MessageResponse
	if res := candidate.sendFile(made.MatchID, "My portfolio", "m2", "portfolio.pdf", []byte("%PDF-1.4 synthetic"), &reply); res.StatusCode != http.StatusOK {
		t.Fatalf("sending a file: status %d", res.StatusCode)
	}
	if reply.File == nil || reply.File.ContentType != "application/pdf" || reply.File.IsImage {
		t.Fatalf("file message = %+v", reply)
	}
	if res := candidate.sendFile(made.MatchID, "", "m3", "run.exe", []byte("MZ binary"), nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("an executable: status %d, want 400", res.StatusCode)
	}
	waitEvent(t, empSock, "message")
	res, _ := employer.http.Get(employer.base + reply.File.URL)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Disposition") != `attachment; filename=portfolio.pdf` {
		t.Errorf("file download: status %d, disposition %q", res.StatusCode, res.Header.Get("Content-Disposition"))
	}
	stranger := *employer
	stranger.http = &http.Client{Jar: newJar()}
	stranger.signInAs("chat.stranger@example.com", "employer")
	if res, _ := stranger.http.Get(stranger.base + reply.File.URL); res.StatusCode != http.StatusNotFound {
		t.Errorf("a stranger's download: status %d, want 404", res.StatusCode)
	}
	if res := stranger.do("GET", chatPath+"/messages", nil, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("a stranger reading the chat: status %d, want 404", res.StatusCode)
	}

	// The inbox, then reading the chat.
	var inbox []MatchResponse
	candidate.do("GET", "/v1/matches", nil, &inbox)
	if len(inbox) != 1 || !inbox[0].New || inbox[0].Unread != 1 || inbox[0].LastMessage == nil || inbox[0].LastMessage.ID != reply.ID {
		t.Errorf("candidate's inbox = %+v", inbox)
	}
	if res := candidate.do("POST", chatPath+"/read", nil, nil); res.StatusCode != http.StatusNoContent {
		t.Errorf("mark read: status %d", res.StatusCode)
	}
	if e := waitEvent(t, empSock, "read"); e.ReadAt == nil {
		t.Error("read event without a time")
	}
	var page MessagesResponse
	employer.do("GET", chatPath+"/messages", nil, &page)
	if len(page.Messages) != 2 || page.Messages[0].ID != hello.ID || page.OtherReadAt == nil || page.HasMore {
		t.Errorf("employer's chat = %+v", page)
	}
	if candidate.do("GET", "/v1/unread", nil, &counts); counts != (UnreadResponse{}) {
		t.Errorf("candidate's unread after reading = %+v, want none", counts)
	}

	// Deleting: only your own messages.
	if res := employer.do("DELETE", "/v1/messages/"+reply.ID, nil, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("deleting the other person's message: status %d, want 404", res.StatusCode)
	}
	if res := candidate.do("DELETE", "/v1/messages/"+reply.ID, nil, nil); res.StatusCode != http.StatusNoContent {
		t.Errorf("deleting your own message: status %d, want 204", res.StatusCode)
	}
	if e := waitEvent(t, empSock, "message_deleted"); e.MessageID != reply.ID {
		t.Errorf("deleted event for %q", e.MessageID)
	}
	var after MessagesResponse
	employer.do("GET", chatPath+"/messages", nil, &after)
	if gone := after.Messages[1]; !gone.Deleted || gone.Body != "" || gone.File != nil {
		t.Errorf("a deleted message shows %+v", gone)
	}
	if res, _ := employer.http.Get(employer.base + reply.File.URL); res.StatusCode != http.StatusNotFound {
		t.Errorf("file of a deleted message: status %d, want 404", res.StatusCode)
	}

	// The employer ends the match: the chat closes for both.
	if res := employer.do("DELETE", chatPath, nil, nil); res.StatusCode != http.StatusNoContent {
		t.Fatalf("employer unmatch: status %d", res.StatusCode)
	}
	waitEvent(t, candSock, "unmatched")
	if res := candidate.do("POST", chatPath+"/messages", map[string]string{"body": "Still there?", "client_id": "m4"}, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("message after unmatch: status %d, want 404", res.StatusCode)
	}
	var kept int64
	employer.tx.Raw("SELECT count(*) FROM messages WHERE match_id = ?", made.MatchID).Scan(&kept)
	if kept != 2 {
		t.Errorf("%d messages kept after unmatch, want 2", kept)
	}
}
