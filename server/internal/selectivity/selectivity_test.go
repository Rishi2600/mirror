package selectivity

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"gorm.io/gorm"

	"mirror/server/internal/dbtest"
	"mirror/server/internal/hiring"
	"mirror/server/internal/users"
)

func near(a, b float64) bool { return math.Abs(a-b) < 0.001 }

// The worked examples in the paper, section 5.
func TestScoreWorkedExamples(t *testing.T) {
	cases := []struct {
		name        string
		in          Inputs
		score, wt   float64
		dailyBudget int
	}{
		{"careful", Inputs{0.25, 0.85, 6000, 0.8}, 0.9225, 0.961, 20},
		{"likes everything", Inputs{0.95, 0.55, 800, 0.5}, 0.3025, 0.651, 15},
		{"in between", Inputs{0.50, 0.70, 2500, 0.5}, 0.65, 0.825, 15},
	}
	for _, c := range cases {
		s := Score(c.in)
		if !near(s, c.score) || !near(Weight(s), c.wt) || DailyBudget(s) != c.dailyBudget {
			t.Errorf("%s: score %.4f, weight %.3f, budget %d; want %.4f, %.3f, %d",
				c.name, s, Weight(s), DailyBudget(s), c.score, c.wt, c.dailyBudget)
		}
	}
	// The paper's limit: every input at its worst or best.
	if got := Score(Inputs{1, 0, 0, 0}); !near(got, 0) {
		t.Errorf("worst score = %f, want 0", got)
	}
	if got := Score(Inputs{0.1, 1, 10000, 1}); !near(got, 1) {
		t.Errorf("best score = %f, want 1", got)
	}
}

func TestLocalDay(t *testing.T) {
	now := time.Date(2026, 10, 30, 20, 0, 0, 0, time.UTC) // 01:30 on the 31st in Kolkata
	start, next := localDay("Asia/Kolkata", now)
	if want := time.Date(2026, 10, 30, 18, 30, 0, 0, time.UTC); !start.Equal(want) {
		t.Errorf("Kolkata day starts %v, want %v", start, want)
	}
	if next.Sub(start) != 24*time.Hour {
		t.Errorf("Kolkata day lasts %v", next.Sub(start))
	}
	if start, _ := localDay("Not/AZone", now); !start.Equal(time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("an unknown zone should mean UTC, got %v", start)
	}
	// The clocks go back on 1 November 2026 in New York: a 25-hour day.
	start, next = localDay("America/New_York", time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
	if next.Sub(start) != 25*time.Hour {
		t.Errorf("New York's day lasts %v, want 25h", next.Sub(start))
	}
}

type world struct {
	ctx       context.Context
	db        *gorm.DB
	employer  users.User
	candidate users.User
	cards     int
	now       time.Time
}

func newWorld(t *testing.T) *world {
	t.Helper()
	ctx := context.Background()
	db := dbtest.Tx(t)
	e, _ := users.Create(ctx, db, "selectivity.e@example.com", users.RoleEmployer)
	c, _ := users.Create(ctx, db, "selectivity.c@example.com", users.RoleCandidate)
	return &world{ctx: ctx, db: db, employer: e, candidate: c, now: time.Now()}
}

func (w *world) card(t *testing.T) string {
	t.Helper()
	w.cards++
	c := hiring.Card{EmployerUserID: w.employer.ID, Status: hiring.StatusLive, Title: fmt.Sprint("Card ", w.cards), Skills: []string{}, PayCurrency: "USD"}
	if err := w.db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	return c.ID
}

// swipes stores n candidate swipes directly, each on a new card.
func (w *world) swipes(t *testing.T, n int, direction string, dwellMS int, fit float64, at time.Time) {
	t.Helper()
	for range n {
		err := w.db.Exec(`INSERT INTO swipes (side, actor_user_id, hiring_card_id, candidate_user_id, direction, idempotency_key, dwell_ms, fit, created_at)
			VALUES ('candidate', ?, ?, ?, ?, gen_random_uuid()::text, ?, ?, ?)`,
			w.candidate.ID, w.card(t), w.candidate.ID, direction, dwellMS, fit, at).Error
		if err != nil {
			t.Fatal(err)
		}
	}
}

func (w *world) state(t *testing.T) State {
	t.Helper()
	if err := AfterSwipe(w.ctx, w.db, w.candidate.ID, w.now); err != nil {
		t.Fatal(err)
	}
	s, err := load(w.ctx, w.db, w.candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestANewCandidateHoldsTheStartingScore(t *testing.T) {
	w := newWorld(t)
	w.swipes(t, newSwipes-1, "right", 300, 0.3, w.now.Add(-time.Hour))
	if s := w.state(t); !near(s.Score, NewScore) {
		t.Errorf("score after %d swipes = %f, want %f", newSwipes-1, s.Score, NewScore)
	}
}

func TestTheScoreComesFromTheLastSevenDays(t *testing.T) {
	w := newWorld(t)
	// Candidate B of the paper's examples, with old skips that must not count.
	w.swipes(t, 57, "right", 800, 0.55, w.now.Add(-48*time.Hour))
	w.swipes(t, 3, "left", 800, 0.55, w.now.Add(-48*time.Hour))
	w.swipes(t, 20, "left", 9000, 0.9, w.now.Add(-8*24*time.Hour))

	s := w.state(t)
	if s.Swipes7d != 60 || !near(s.RightRate, 0.95) || !near(s.MeanFit, 0.55) || !near(s.MedianDwellMS, 800) || !near(s.ReplyRate, 0.5) {
		t.Errorf("inputs = %+v", s)
	}
	if !near(s.Score, 0.3025) {
		t.Errorf("score = %f, want 0.3025", s.Score)
	}
	var snapshots int64
	w.db.Raw("SELECT count(*) FROM events.selectivity_snapshots WHERE user_id = ?", w.candidate.ID).Scan(&snapshots)
	if snapshots != 1 {
		t.Errorf("%d snapshots, want 1", snapshots)
	}
	w.state(t) // nothing changed
	w.db.Raw("SELECT count(*) FROM events.selectivity_snapshots WHERE user_id = ?", w.candidate.ID).Scan(&snapshots)
	if snapshots != 1 {
		t.Errorf("%d snapshots after no change, want still 1", snapshots)
	}
}

func TestAQuietWeekKeepsTheLastScore(t *testing.T) {
	w := newWorld(t)
	w.swipes(t, 60, "right", 800, 0.55, w.now.Add(-8*24*time.Hour))
	w.db.Create(&State{UserID: w.candidate.ID, Score: 0.9, UpdatedAt: w.now.Add(-8 * 24 * time.Hour)})
	w.swipes(t, 1, "right", 800, 0.55, w.now.Add(-time.Minute))
	if s := w.state(t); !near(s.Score, 0.9) {
		t.Errorf("score after one swipe this week = %f, want the last score 0.9", s.Score)
	}
}

// match stores a match made this long ago, and a message from the candidate
// or employer sent after the given time (a negative time means no message).
func (w *world) match(t *testing.T, who users.User, age, wroteAfter time.Duration, ended bool) {
	t.Helper()
	var id string
	matched := w.now.Add(-age)
	var unmatched *time.Time
	if ended {
		at := matched.Add(time.Hour)
		unmatched = &at
	}
	err := w.db.Raw(`INSERT INTO matches (hiring_card_id, candidate_user_id, employer_user_id, matched_at, unmatched_at)
		VALUES (?, ?, ?, ?, ?) RETURNING id`, w.card(t), w.candidate.ID, w.employer.ID, matched, unmatched).Scan(&id).Error
	if err != nil {
		t.Fatal(err)
	}
	if wroteAfter >= 0 {
		w.db.Exec(`INSERT INTO messages (match_id, sender_user_id, body, client_id, created_at) VALUES (?, ?, 'Hello', 'c1', ?)`,
			id, who.ID, matched.Add(wroteAfter))
	}
}

func TestReplyRate(t *testing.T) {
	w := newWorld(t)
	day := 24 * time.Hour
	if got, _ := replyRate(w.ctx, w.db, w.candidate.ID, w.now); got != noReplyData {
		t.Errorf("with no matches: %f, want %f", got, noReplyData)
	}
	w.match(t, w.candidate, 5*day, time.Hour, false) // replied
	w.match(t, w.candidate, 5*day, 4*day, false)     // replied too late
	w.match(t, w.candidate, 5*day, -1, false)        // never replied
	w.match(t, w.candidate, 1*day, -1, false)        // too new to judge
	w.match(t, w.candidate, 5*day, -1, true)         // ended within hours: no fair chance
	w.match(t, w.candidate, 11*day, -1, false)       // outside the 7 days
	if got, _ := replyRate(w.ctx, w.db, w.candidate.ID, w.now); !near(got, 1.0/3) {
		t.Errorf("reply rate = %f, want 1/3", got)
	}
}

func TestDailyBudget(t *testing.T) {
	w := newWorld(t)
	now := time.Date(2026, 10, 30, 20, 0, 0, 0, time.UTC) // 01:30 on the 31st in Kolkata
	w.candidate.TimeZone = "Asia/Kolkata"
	w.swipes(t, 2, "right", 5000, 0.8, time.Date(2026, 10, 30, 18, 0, 0, 0, time.UTC)) // 23:30 yesterday there
	w.swipes(t, 14, "right", 5000, 0.8, time.Date(2026, 10, 30, 19, 0, 0, 0, time.UTC))
	w.swipes(t, 5, "left", 5000, 0.8, time.Date(2026, 10, 30, 19, 0, 0, 0, time.UTC))

	likes, err := LikesFor(w.ctx, w.db, w.candidate, now)
	if err != nil || likes.Left != 1 || likes.DailyBudget != 15 {
		t.Fatalf("likes = %+v, %v; want 1 left of 15 (skips and yesterday not counted)", likes, err)
	}
	if err := CheckLike(w.ctx, w.db, w.candidate, now); err != nil {
		t.Errorf("the last like was refused: %v", err)
	}

	w.swipes(t, 1, "right", 5000, 0.8, time.Date(2026, 10, 30, 19, 30, 0, 0, time.UTC))
	var limited *Limited
	if err := CheckLike(w.ctx, w.db, w.candidate, now); !errors.As(err, &limited) {
		t.Fatalf("over budget: error = %v, want *Limited", err)
	}
	if want := time.Date(2026, 10, 31, 18, 30, 0, 0, time.UTC); !limited.Until.Equal(want) {
		t.Errorf("resets at %v, want local midnight %v", limited.Until, want)
	}

	// The same likes in UTC are all from today, so a careful candidate with
	// 20 likes a day has 3 left.
	w.candidate.TimeZone = ""
	w.db.Create(&State{UserID: w.candidate.ID, Score: 0.85, UpdatedAt: now})
	if likes, _ := LikesFor(w.ctx, w.db, w.candidate, now); likes.Left != 3 || likes.DailyBudget != 20 {
		t.Errorf("likes = %+v, want 3 left of 20", likes)
	}
}

func TestBurstPause(t *testing.T) {
	cases := []struct {
		name    string
		dwellMS int
		spread  time.Duration
		paused  bool
	}{
		{"fast and unread", 800, 40 * time.Second, true},
		{"fast but read", 5000, 50 * time.Second, false},
		{"unread but slow", 800, 2 * time.Minute, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			for i := range burstLikes {
				w.swipes(t, 1, "right", c.dwellMS, 0.6, w.now.Add(-c.spread+time.Duration(i)*c.spread/(burstLikes-1)))
			}
			s := w.state(t)
			if got := s.PausedUntil != nil; got != c.paused {
				t.Fatalf("paused = %v, want %v", got, c.paused)
			}
			if !c.paused {
				return
			}
			if want := w.now.Add(burstPause); !s.PausedUntil.Equal(want) && s.PausedUntil.Sub(want).Abs() > time.Millisecond {
				t.Errorf("paused until %v, want %v", s.PausedUntil, want)
			}
			var limited *Limited
			if err := CheckLike(w.ctx, w.db, w.candidate, w.now); !errors.As(err, &limited) {
				t.Errorf("a like during the pause: error = %v, want *Limited", err)
			}
			if err := CheckLike(w.ctx, w.db, w.candidate, w.now.Add(burstPause+time.Second)); err != nil {
				t.Errorf("a like after the pause: %v", err)
			}
		})
	}
}

func TestSevenLikesAreNotABurst(t *testing.T) {
	w := newWorld(t)
	w.swipes(t, burstLikes-1, "right", 300, 0.6, w.now.Add(-10*time.Second))
	if s := w.state(t); s.PausedUntil != nil {
		t.Error("7 likes set a pause")
	}
}

func TestLowScoreLastsUntilTheScoreRecovers(t *testing.T) {
	w := newWorld(t)
	w.swipes(t, 60, "right", 300, 0.3, w.now.Add(-time.Hour))
	first := w.state(t)
	if first.Score >= lowScore || first.LowSince == nil {
		t.Fatalf("state = %+v, want a low score with low_since set", first)
	}
	if likes, _ := LikesFor(w.ctx, w.db, w.candidate, w.now); likes.LowScore {
		t.Error("low score rule applied on day 1")
	}
	if likes, _ := LikesFor(w.ctx, w.db, w.candidate, first.LowSince.Add(LowAfter)); !likes.LowScore {
		t.Error("low score rule not applied after 3 days")
	}

	again := w.state(t)
	if !again.LowSince.Equal(*first.LowSince) {
		t.Error("low_since moved while the score stayed low: the 3 days would never pass")
	}

	w.db.Exec("UPDATE swipes SET direction = 'left', dwell_ms = 9000 WHERE actor_user_id = ?", w.candidate.ID)
	if s := w.state(t); s.Score < lowScore || s.LowSince != nil {
		t.Errorf("after better swiping: %+v, want low_since cleared", s)
	}
}

func TestGhosting(t *testing.T) {
	day := 24 * time.Hour
	cases := []struct {
		name    string
		replied int
		ignored int
		want    bool
	}{
		{"ignores every match", 0, 3, true},
		{"replies to 1 in 4 (25%)", 1, 3, true},
		{"replies to 1 in 3 (33%)", 1, 2, false},
		{"too few matches to judge", 0, 2, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			for range c.replied {
				w.match(t, w.employer, 5*day, time.Hour, false)
			}
			for range c.ignored {
				w.match(t, w.employer, 5*day, -1, false)
			}
			got, err := Ghosting(w.ctx, w.db, []string{w.employer.ID}, w.now)
			if err != nil || got[w.employer.ID] != c.want {
				t.Errorf("ghosting = %v, %v; want %v", got, err, c.want)
			}
		})
	}
}

func TestACandidateMessageIsNotAnEmployerReply(t *testing.T) {
	w := newWorld(t)
	for range 3 {
		w.match(t, w.candidate, 5*24*time.Hour, time.Hour, false)
	}
	if got, _ := Ghosting(w.ctx, w.db, []string{w.employer.ID}, w.now); !got[w.employer.ID] {
		t.Error("the candidate's messages counted as the employer's replies")
	}
}
