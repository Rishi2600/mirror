package httpserver

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"mirror/server/internal/chat"
	"mirror/server/internal/events"
	"mirror/server/internal/live"
	"mirror/server/internal/swipes"
	"mirror/server/internal/users"
)

// MessageResponse is one chat message. A deleted message has no body or
// file.
type MessageResponse struct {
	ID        string    `json:"id"`
	MatchID   string    `json:"match_id"`
	SenderID  string    `json:"sender_id"`
	ClientID  string    `json:"client_id"`
	Body      string    `json:"body"`
	File      *FileInfo `json:"file,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Deleted   bool      `json:"deleted"`
}

// FileInfo describes a message's attachment. Images are shown inline.
type FileInfo struct {
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	IsImage     bool   `json:"is_image"`
	URL         string `json:"url"`
}

// MessagesResponse is one page of a chat, oldest first.
type MessagesResponse struct {
	Messages []MessageResponse `json:"messages"`
	HasMore  bool              `json:"has_more"`
	// OtherReadAt is when the other person last read the chat (for "Seen").
	OtherReadAt *time.Time `json:"other_read_at"`
}

// UnreadResponse is what the home badge counts.
type UnreadResponse struct {
	NewMatches int `json:"new_matches"`
	Messages   int `json:"messages"`
}

func toMessageResponse(m chat.Message) MessageResponse {
	out := MessageResponse{ID: m.ID, MatchID: m.MatchID, SenderID: m.SenderUserID, ClientID: m.ClientID, CreatedAt: m.CreatedAt}
	if m.DeletedAt != nil {
		out.Deleted = true
		return out
	}
	out.Body = m.Body
	if m.File != nil {
		out.File = &FileInfo{
			Name: m.FileName, ContentType: m.File.ContentType, Size: m.File.SizeBytes,
			IsImage: chat.IsImage(m.File), URL: "/v1/messages/" + m.ID + "/file",
		}
	}
	return out
}

// GET /v1/matches/{id}/messages?before=<message id>: one page of the chat.
func (s *server) listMessages(w http.ResponseWriter, r *http.Request) {
	user, m, ok := s.ownMatch(w, r)
	if !ok {
		return
	}
	list, more, err := chat.History(r.Context(), s.db, m.ID, r.URL.Query().Get("before"))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	out := MessagesResponse{Messages: []MessageResponse{}, HasMore: more}
	for _, msg := range list {
		out.Messages = append(out.Messages, toMessageResponse(msg))
	}
	_, out.OtherReadAt = m.ReadAt(user.ID)
	writeJSON(w, http.StatusOK, out)
}

// POST /v1/matches/{id}/messages: JSON {body, client_id}, or a multipart
// form with the same fields and the attachment in "file".
func (s *server) sendMessage(w http.ResponseWriter, r *http.Request) {
	user, m, ok := s.ownMatch(w, r)
	if !ok {
		return
	}
	var body, clientID string
	var upload *chat.Upload
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, chat.MaxFileBytes+(1<<20))
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "send the attachment as a form file named file (up to 10 MB)")
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, chat.MaxFileBytes+1))
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		if len(data) > chat.MaxFileBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "a file must be 10 MB or smaller")
			return
		}
		upload = &chat.Upload{Name: header.Filename, Data: data}
		body, clientID = r.FormValue("body"), r.FormValue("client_id")
	} else {
		var in struct {
			Body     string `json:"body"`
			ClientID string `json:"client_id"`
		}
		if !readJSON(w, r, &in) {
			return
		}
		body, clientID = in.Body, in.ClientID
	}

	msg, err := chat.Send(r.Context(), s.db, s.files, m, user.ID, body, clientID, upload)
	switch {
	case errors.Is(err, chat.ErrClientID), errors.Is(err, chat.ErrEmpty), errors.Is(err, chat.ErrTooLong), errors.Is(err, chat.ErrFileType):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case err != nil:
		s.serverError(w, r, err)
		return
	}
	resp := toMessageResponse(msg)
	e := live.Event{Type: live.TypeMessage, MatchID: m.ID, Message: resp}
	s.hub.Send(user.ID, e) // the sender's other tabs
	s.notify(r, m.OtherUserID(user.ID), "message", m.ID, e)
	writeJSON(w, http.StatusOK, resp)
}

// POST /v1/matches/{id}/read: the user has read the chat up to now.
func (s *server) markRead(w http.ResponseWriter, r *http.Request) {
	user, m, ok := s.ownMatch(w, r)
	if !ok {
		return
	}
	at, err := chat.MarkRead(r.Context(), s.db, m, user.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.hub.Send(m.OtherUserID(user.ID), live.Event{Type: live.TypeRead, MatchID: m.ID, ReadAt: &at})
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /v1/messages/{id}: the sender hides their own message.
func (s *server) deleteMessage(w http.ResponseWriter, r *http.Request) {
	user, m, ok := s.messageMatch(w, r)
	if !ok {
		return
	}
	msg, err := chat.Delete(r.Context(), s.db, m.ID, user.ID, r.PathValue("id"))
	if errors.Is(err, chat.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	e := live.Event{Type: live.TypeMessageDeleted, MatchID: m.ID, MessageID: msg.ID}
	s.hub.Send(m.CandidateUserID, e)
	s.hub.Send(m.EmployerUserID, e)
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/messages/{id}/file: a message's attachment, for the two people in
// the match.
func (s *server) messageFile(w http.ResponseWriter, r *http.Request) {
	_, m, ok := s.messageMatch(w, r)
	if !ok {
		return
	}
	msg, data, err := chat.FileOf(r.Context(), s.db, s.files, m.ID, r.PathValue("id"))
	if errors.Is(err, chat.ErrNotFound) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	disposition := "attachment"
	if chat.IsImage(msg.File) {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", msg.File.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": msg.FileName}))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Write(data)
}

// GET /v1/unread: new matches and unread messages, for the home badge.
func (s *server) unread(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	matches, messages, err := chat.Unread(r.Context(), s.db, user.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, UnreadResponse{NewMatches: matches, Messages: messages})
}

// messageMatch loads the live match of the message in the path, if the user
// is part of it.
func (s *server) messageMatch(w http.ResponseWriter, r *http.Request) (users.User, swipes.Match, bool) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return users.User{}, swipes.Match{}, false
	}
	matchID, err := chat.MatchOf(r.Context(), s.db, r.PathValue("id"))
	if err == nil {
		var m swipes.Match
		m, err = swipes.Get(r.Context(), s.db, user, matchID)
		if err == nil {
			return user, m, true
		}
	}
	if errors.Is(err, chat.ErrNotFound) || errors.Is(err, swipes.ErrMatchNotFound) {
		writeError(w, http.StatusNotFound, chat.ErrNotFound.Error())
	} else {
		s.serverError(w, r, err)
	}
	return users.User{}, swipes.Match{}, false
}

// notifyMatch tells both sides about a new match. Only the other side is
// notified: the swiper sees it in the swipe's answer.
func (s *server) notifyMatch(r *http.Request, swiper users.User, m swipes.Match) {
	e := live.Event{Type: live.TypeMatch, MatchID: m.ID}
	s.hub.Send(swiper.ID, e)
	s.notify(r, m.OtherUserID(swiper.ID), "match", m.ID, e)
}

// notify pushes an in-app notification and logs it.
func (s *server) notify(r *http.Request, userID, kind, matchID string, e live.Event) {
	delivered := s.hub.Send(userID, e)
	n := events.Notification{UserID: userID, Channel: "in_app", Kind: kind, MatchID: matchID, Delivered: delivered}
	if err := events.LogNotification(r.Context(), s.db, n); err != nil {
		s.logger.Error("logging a notification", "err", err)
	}
}
