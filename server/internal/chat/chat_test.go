package chat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"mirror/server/internal/dbtest"
	"mirror/server/internal/hiring"
	"mirror/server/internal/storage"
	"mirror/server/internal/swipes"
	"mirror/server/internal/users"
)

type fixture struct {
	ctx       context.Context
	db        *gorm.DB
	files     *storage.Local
	candidate users.User
	employer  users.User
	match     swipes.Match
}

// newFixture makes a candidate and an employer with a live match.
func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	db := dbtest.Tx(t)
	files, err := storage.NewLocal(t.TempDir(), bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	c, _ := users.Create(ctx, db, "chat.c@example.com", users.RoleCandidate)
	e, _ := users.Create(ctx, db, "chat.e@example.com", users.RoleEmployer)
	card := hiring.Card{EmployerUserID: e.ID, Status: hiring.StatusLive, Title: "Go engineer", Skills: []string{}, PayCurrency: "USD"}
	if err := db.Create(&card).Error; err != nil {
		t.Fatal(err)
	}
	m := swipes.Match{HiringCardID: card.ID, CandidateUserID: c.ID, EmployerUserID: e.ID, MatchedAt: time.Now()}
	if err := db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	return fixture{ctx, db, files, c, e, m}
}

func (f fixture) send(t *testing.T, from users.User, body, clientID string, up *Upload) Message {
	t.Helper()
	msg, err := Send(f.ctx, f.db, f.files, f.match, from.ID, body, clientID, up)
	if err != nil {
		t.Fatalf("Send(%q): %v", body, err)
	}
	return msg
}

func TestSendChecksTheMessage(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		body, clientID string
		up             *Upload
		want           error
	}{
		{"hello", "", nil, ErrClientID},
		{"   ", "k1", nil, ErrEmpty},
		{strings.Repeat("a", MaxBody+1), "k2", nil, ErrTooLong},
		{"", "k3", &Upload{Name: "notes.txt", Data: []byte("plain text")}, ErrFileType},
	}
	for _, c := range cases {
		if _, err := Send(f.ctx, f.db, f.files, f.match, f.candidate.ID, c.body, c.clientID, c.up); !errors.Is(err, c.want) {
			t.Errorf("Send(%.10q): error = %v, want %v", c.body, err, c.want)
		}
	}
	if msg := f.send(t, f.candidate, strings.Repeat("é", MaxBody), "k4", nil); msg.ID == "" {
		t.Error("4000 two-byte characters were refused")
	}
}

func TestARetriedSendSavesOneMessage(t *testing.T) {
	f := newFixture(t)
	first := f.send(t, f.candidate, " Hi there ", "same", nil)
	again := f.send(t, f.candidate, "Hi there", "same", nil)
	if first.ID != again.ID || first.Body != "Hi there" {
		t.Errorf("first %+v, again %+v; want one trimmed message", first, again)
	}
	list, _, _ := History(f.ctx, f.db, f.match.ID, "")
	if len(list) != 1 {
		t.Errorf("history has %d messages, want 1", len(list))
	}
}

func TestFilesAreCheckedAndImagesCleaned(t *testing.T) {
	f := newFixture(t)
	var img bytes.Buffer
	png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 3000, 1500)))

	photo := f.send(t, f.employer, "", "img", &Upload{Name: `C:\Users\sam\office photo.png`, Data: img.Bytes()})
	if photo.FileName != "office photo.jpg" || !IsImage(photo.File) {
		t.Errorf("image message: name %q, file %+v", photo.FileName, photo.File)
	}
	_, data, err := FileOf(f.ctx, f.db, f.files, f.match.ID, photo.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || stored.Width != maxImageSide || stored.Height != maxImageSide/2 {
		t.Errorf("stored image %dx%d (%v), want %dx%d", stored.Width, stored.Height, err, maxImageSide, maxImageSide/2)
	}

	doc := f.send(t, f.employer, "The offer", "pdf", &Upload{Name: "offer.pdf", Data: []byte("%PDF-1.4 synthetic")})
	if doc.File.ContentType != "application/pdf" || IsImage(doc.File) || doc.Body != "The offer" {
		t.Errorf("document message: %+v, file %+v", doc, doc.File)
	}
}

func TestHistoryPagesBackwards(t *testing.T) {
	f := newFixture(t)
	for i := range PageSize + 5 {
		f.db.Exec("INSERT INTO messages (match_id, sender_user_id, body, client_id, created_at) VALUES (?, ?, ?, ?, ?)",
			f.match.ID, f.candidate.ID, fmt.Sprint(i), fmt.Sprint("c", i), time.Now().Add(time.Duration(i)*time.Second))
	}
	page, more, err := History(f.ctx, f.db, f.match.ID, "")
	if err != nil || len(page) != PageSize || !more {
		t.Fatalf("first page: %d messages, more %v, %v", len(page), more, err)
	}
	if page[0].Body != "5" || page[len(page)-1].Body != fmt.Sprint(PageSize+4) {
		t.Errorf("first page runs %s..%s, want 5..%d oldest first", page[0].Body, page[len(page)-1].Body, PageSize+4)
	}
	older, more, _ := History(f.ctx, f.db, f.match.ID, page[0].ID)
	if len(older) != 5 || more || older[0].Body != "0" || older[4].Body != "4" {
		t.Errorf("older page: %d messages (%v), more %v", len(older), older, more)
	}
}

func TestOnlyTheSenderDeletes(t *testing.T) {
	f := newFixture(t)
	msg := f.send(t, f.candidate, "oops", "k1", &Upload{Name: "cv.pdf", Data: []byte("%PDF-1.4 synthetic")})
	if _, err := Delete(f.ctx, f.db, f.match.ID, f.employer.ID, msg.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("the other person deleting: error = %v, want ErrNotFound", err)
	}
	gone, err := Delete(f.ctx, f.db, f.match.ID, f.candidate.ID, msg.ID)
	if err != nil || gone.DeletedAt == nil {
		t.Fatalf("Delete: %+v, %v", gone, err)
	}
	if gone.Body != "oops" {
		t.Error("the deleted message's text should stay in the database for moderation")
	}
	if _, _, err := FileOf(f.ctx, f.db, f.files, f.match.ID, msg.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("file of a deleted message: error = %v, want ErrNotFound", err)
	}
	if _, err := Delete(f.ctx, f.db, f.match.ID, f.candidate.ID, msg.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleting twice: error = %v, want ErrNotFound", err)
	}
}

func TestUnreadCountsAndReadMarks(t *testing.T) {
	f := newFixture(t)
	counts := func(u users.User) string {
		n, m, err := Unread(f.ctx, f.db, u.ID)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%d new, %d unread", n, m)
	}
	if got := counts(f.candidate); got != "1 new, 0 unread" {
		t.Errorf("before anything: %s", got)
	}

	f.send(t, f.employer, "Hello", "e1", nil)
	f.send(t, f.employer, "Are you free on Monday?", "e2", nil)
	deleted := f.send(t, f.employer, "typo", "e3", nil)
	Delete(f.ctx, f.db, f.match.ID, f.employer.ID, deleted.ID)
	if got := counts(f.candidate); got != "1 new, 2 unread" {
		t.Errorf("after 2 messages and a deleted one: %s", got)
	}
	if got := counts(f.employer); got != "1 new, 0 unread" {
		t.Errorf("the sender's own messages counted: %s", got)
	}

	sums, _ := Summaries(f.ctx, f.db, f.candidate.ID, []string{f.match.ID})
	if s := sums[f.match.ID]; s.Unread != 2 || s.Last == nil || s.Last.ID != deleted.ID {
		t.Errorf("summary = %+v, want 2 unread and the latest message", s)
	}

	time.Sleep(10 * time.Millisecond) // read after the messages' timestamps
	if _, err := MarkRead(f.ctx, f.db, f.match, f.candidate.ID); err != nil {
		t.Fatal(err)
	}
	if got := counts(f.candidate); got != "0 new, 0 unread" {
		t.Errorf("after reading: %s", got)
	}
}
