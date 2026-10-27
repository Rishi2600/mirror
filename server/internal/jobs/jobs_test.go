package jobs

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"mirror/server/internal/dbtest"
	"mirror/server/internal/profiles"
	"mirror/server/internal/storage"
	"mirror/server/internal/users"
)

type failingParser struct{}

func (failingParser) ParseCV(ctx context.Context, text string) (profiles.CV, error) {
	return profiles.CV{}, errors.New("groq is down")
}

// After the last try, the profile is marked failed so the candidate is not
// left waiting forever.
func TestLastFailedTryMarksProfileFailed(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.Tx(t)
	files, _ := storage.NewLocal(t.TempDir(), bytes.Repeat([]byte{1}, 32))
	u, _ := users.Create(ctx, tx, "jobs@example.com", users.RoleCandidate)
	cv, _ := os.ReadFile("../cvtext/testdata/sample-cv.pdf")
	_, fileID, err := profiles.UploadCV(ctx, tx, files, u.ID, cv)
	if err != nil {
		t.Fatal(err)
	}

	w := &parseCVWorker{db: tx, files: files, parser: failingParser{}}
	job := func(attempt int) *river.Job[ParseCVArgs] {
		return &river.Job[ParseCVArgs]{
			JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: 3},
			Args:   ParseCVArgs{UserID: u.ID, FileID: fileID},
		}
	}

	if err := w.Work(ctx, job(1)); err == nil {
		t.Fatal("Work hid the AI error")
	}
	if p, _ := profiles.Get(ctx, tx, u.ID); p.ParseStatus != profiles.StatusPending {
		t.Errorf("after try 1: status %q, want pending (River will retry)", p.ParseStatus)
	}

	w.Work(ctx, job(3))
	if p, _ := profiles.Get(ctx, tx, u.ID); p.ParseStatus != profiles.StatusFailed || p.ParseError == "" {
		t.Errorf("after the last try: profile %+v, want failed with a message", p)
	}
}
