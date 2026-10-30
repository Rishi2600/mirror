package swipes

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"mirror/server/internal/dbtest"
	"mirror/server/internal/hiring"
	"mirror/server/internal/profiles"
	"mirror/server/internal/selectivity"
	"mirror/server/internal/users"
)

type pair struct {
	ctx       context.Context
	db        *gorm.DB
	candidate users.User
	employer  users.User
	card      hiring.Card
}

// newPair makes a finished candidate and an employer with one live card.
func newPair(t *testing.T, db *gorm.DB, tag string) pair {
	t.Helper()
	ctx := context.Background()
	c, err := users.Create(ctx, db, "swipes.c."+tag+"@example.com", users.RoleCandidate)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := users.Create(ctx, db, "swipes.e."+tag+"@example.com", users.RoleEmployer)
	now := time.Now()
	p := profiles.Profile{UserID: c.ID, ParseStatus: profiles.StatusDone, OnboardedAt: &now, WorkMode: "any", PayCurrency: "USD",
		EmployerDisplay: "name", RoleTypes: []string{}, Locations: []string{}, HiddenFrom: []string{}}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	card := hiring.Card{EmployerUserID: e.ID, Status: hiring.StatusLive, Title: "Go engineer", Skills: []string{}, PayCurrency: "USD"}
	if err := db.Create(&card).Error; err != nil {
		t.Fatal(err)
	}
	return pair{ctx, db, c, e, card}
}

func (p pair) swipe(t *testing.T, actor users.User, direction, key string) (Result, error) {
	t.Helper()
	return Record(p.ctx, p.db, actor, p.card.ID, p.candidate.ID, direction, key, 5000)
}

func TestTwoRightSwipesMakeOneMatch(t *testing.T) {
	p := newPair(t, dbtest.Tx(t), "match")

	res, err := p.swipe(t, p.candidate, Right, "k1")
	if err != nil || res.Match != nil {
		t.Fatalf("first like: %+v, %v; want a swipe and no match", res, err)
	}
	res, err = p.swipe(t, p.employer, Right, "k2")
	if err != nil || res.Match == nil {
		t.Fatalf("second like: %+v, %v; want a match", res, err)
	}
	if res.Match.EmployerUserID != p.employer.ID || res.Match.State != StateMatched {
		t.Errorf("match = %+v", res.Match)
	}

	var reveals int64
	p.db.Table("audit_events").Where("match_id = ? AND action = 'reveal'", res.Match.ID).Count(&reveals)
	if reveals != 1 {
		t.Errorf("%d reveal audit events, want 1", reveals)
	}
}

func TestLeftSwipeMakesNoMatch(t *testing.T) {
	p := newPair(t, dbtest.Tx(t), "left")
	p.swipe(t, p.candidate, Right, "k1")
	if res, _ := p.swipe(t, p.employer, Left, "k2"); res.Match != nil {
		t.Error("a like and a skip made a match")
	}
}

func TestOneSwipePerPairAndReplay(t *testing.T) {
	p := newPair(t, dbtest.Tx(t), "once")
	first, err := p.swipe(t, p.candidate, Right, "k1")
	if err != nil {
		t.Fatal(err)
	}

	if again, err := p.swipe(t, p.candidate, Right, "k1"); err != nil || again.Swipe.ID != first.Swipe.ID {
		t.Errorf("replaying the same key = %+v, %v; want the first swipe back", again, err)
	}
	if _, err := p.swipe(t, p.candidate, Left, "k1"); !errors.Is(err, ErrKeyReused) {
		t.Errorf("same key, different swipe: error = %v, want ErrKeyReused", err)
	}
	if _, err := p.swipe(t, p.candidate, Left, "k2"); !errors.Is(err, ErrAlreadySwiped) {
		t.Errorf("second swipe on the pair: error = %v, want ErrAlreadySwiped", err)
	}
}

func TestWhoMaySwipe(t *testing.T) {
	db := dbtest.Tx(t)
	p := newPair(t, db, "who")
	other := newPair(t, db, "other")

	tests := []struct {
		name                string
		actor               users.User
		cardID, candidateID string
		want                error
	}{
		{"candidate for someone else", p.candidate, p.card.ID, other.candidate.ID, ErrNotAllowed},
		{"employer on another employer's card", p.employer, other.card.ID, p.candidate.ID, ErrNotAllowed},
		{"bad direction", p.candidate, p.card.ID, p.candidate.ID, ErrDirection},
		{"no key", p.candidate, p.card.ID, p.candidate.ID, ErrKey},
	}
	for _, tt := range tests {
		direction, key := Right, "k-"+tt.name
		if tt.want == ErrDirection {
			direction = "up"
		}
		if tt.want == ErrKey {
			key = ""
		}
		if _, err := Record(p.ctx, db, tt.actor, tt.cardID, tt.candidateID, direction, key, 5000); !errors.Is(err, tt.want) {
			t.Errorf("%s: error = %v, want %v", tt.name, err, tt.want)
		}
	}

	db.Model(&p.card).Update("status", hiring.StatusClosed)
	if _, err := p.swipe(t, p.candidate, Right, "closed"); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("swipe on a closed card: error = %v, want ErrNotAllowed", err)
	}
}

func TestMatchesAndUnmatch(t *testing.T) {
	p := newPair(t, dbtest.Tx(t), "unmatch")
	p.swipe(t, p.candidate, Right, "k1")
	res, _ := p.swipe(t, p.employer, Right, "k2")

	for _, u := range []users.User{p.candidate, p.employer} {
		if list, _ := Matches(p.ctx, p.db, u); len(list) != 1 {
			t.Errorf("%s sees %d matches, want 1", u.Role, len(list))
		}
		if _, err := Get(p.ctx, p.db, u, res.Match.ID); err != nil {
			t.Errorf("%s Get: %v", u.Role, err)
		}
	}
	stranger, _ := users.Create(p.ctx, p.db, "swipes.stranger@example.com", users.RoleEmployer)
	if _, err := Get(p.ctx, p.db, stranger, res.Match.ID); !errors.Is(err, ErrMatchNotFound) {
		t.Errorf("a stranger's Get: error = %v, want ErrMatchNotFound", err)
	}

	if _, err := Unmatch(p.ctx, p.db, stranger, res.Match.ID); !errors.Is(err, ErrMatchNotFound) {
		t.Errorf("a stranger unmatching: error = %v, want ErrMatchNotFound", err)
	}
	if _, err := Unmatch(p.ctx, p.db, p.employer, res.Match.ID); err != nil {
		t.Fatalf("employer Unmatch: %v", err)
	}
	for _, u := range []users.User{p.candidate, p.employer} {
		if _, err := Get(p.ctx, p.db, u, res.Match.ID); !errors.Is(err, ErrMatchNotFound) {
			t.Errorf("%s after unmatch: error = %v, want ErrMatchNotFound", u.Role, err)
		}
	}
	if _, err := Unmatch(p.ctx, p.db, p.candidate, res.Match.ID); !errors.Is(err, ErrMatchNotFound) {
		t.Errorf("unmatching twice: error = %v, want ErrMatchNotFound", err)
	}
	var by string
	p.db.Raw("SELECT unmatched_by FROM matches WHERE id = ?", res.Match.ID).Scan(&by)
	if by != p.employer.ID {
		t.Errorf("unmatched_by = %q, want the employer", by)
	}
}

// removeUsers deletes test users and their rows. Audit rows and event rows
// outlive their users on purpose, so they go first.
func removeUsers(db *gorm.DB, emails string) {
	db.Exec("DELETE FROM audit_events WHERE actor_user_id IN (SELECT id FROM users WHERE email LIKE ?)", emails)
	db.Exec("DELETE FROM events.selectivity_snapshots WHERE user_id IN (SELECT id FROM users WHERE email LIKE ?)", emails)
	db.Exec("DELETE FROM users WHERE email LIKE ?", emails)
}

// Both sides like the pair at the same moment, many times over: exactly one
// match each time. Uses real transactions, so it removes its own rows.
func TestSimultaneousLikesMakeExactlyOneMatch(t *testing.T) {
	db := dbtest.Conn(t)
	// Remove this test's rows before (in case an earlier run crashed) and after.
	cleanup := func() { removeUsers(db, "swipes._.race.%@example.com") }
	cleanup()
	t.Cleanup(cleanup)

	for i := range 20 {
		p := newPair(t, db, fmt.Sprintf("race.%d", i))
		var wg sync.WaitGroup
		results := make([]Result, 2)
		errs := make([]error, 2)
		for j, actor := range []users.User{p.candidate, p.employer} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[j], errs[j] = p.swipe(t, actor, Right, fmt.Sprintf("race-%d-%d", i, j))
			}()
		}
		wg.Wait()

		if errs[0] != nil || errs[1] != nil {
			t.Fatalf("round %d: errors %v, %v", i, errs[0], errs[1])
		}
		var matches int64
		db.Model(&Match{}).Where("hiring_card_id = ?", p.card.ID).Count(&matches)
		made := 0
		for _, r := range results {
			if r.Match != nil {
				made++
			}
		}
		if matches != 1 || made != 1 {
			t.Fatalf("round %d: %d matches stored, %d reported; want exactly 1 of each", i, matches, made)
		}
	}
}

// fillBudget stores likes on new cards, as if sent earlier today.
func (p pair) fillBudget(t *testing.T, likes int) {
	t.Helper()
	for i := range likes {
		card := hiring.Card{EmployerUserID: p.employer.ID, Status: hiring.StatusLive, Title: fmt.Sprint("Filled ", i), Skills: []string{}, PayCurrency: "USD"}
		if err := p.db.Create(&card).Error; err != nil {
			t.Fatal(err)
		}
		err := p.db.Exec(`INSERT INTO swipes (side, actor_user_id, hiring_card_id, candidate_user_id, direction, idempotency_key, dwell_ms)
			VALUES ('candidate', ?, ?, ?, 'right', gen_random_uuid()::text, 5000)`, p.candidate.ID, card.ID, p.candidate.ID).Error
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestDwellIsStoredOnCandidateSwipes(t *testing.T) {
	p := newPair(t, dbtest.Tx(t), "dwell")
	cases := []struct{ sent, want int }{{-5, 0}, {4200, 4200}}
	for i, c := range cases {
		card := hiring.Card{EmployerUserID: p.employer.ID, Status: hiring.StatusLive, Title: "Go", Skills: []string{}, PayCurrency: "USD"}
		p.db.Create(&card)
		res, err := Record(p.ctx, p.db, p.candidate, card.ID, p.candidate.ID, Left, fmt.Sprint("d", i), c.sent)
		if err != nil || res.Swipe.DwellMS == nil || *res.Swipe.DwellMS != c.want {
			t.Errorf("dwell %d stored as %v (%v), want %d", c.sent, res.Swipe.DwellMS, err, c.want)
		}
	}
	res, _ := Record(p.ctx, p.db, p.candidate, p.card.ID, p.candidate.ID, Right, "long", 3*60*60*1000)
	if *res.Swipe.DwellMS != maxDwellMS {
		t.Errorf("3 hours on a card stored as %d ms, want the cap", *res.Swipe.DwellMS)
	}
	res, _ = p.swipe(t, p.employer, Right, "e1")
	if res.Swipe.DwellMS != nil {
		t.Error("an employer swipe stored a dwell time")
	}
}

func TestARefusedLikeSavesNothingButARetryStillAnswers(t *testing.T) {
	p := newPair(t, dbtest.Tx(t), "refused")
	if _, err := p.swipe(t, p.candidate, Right, "first"); err != nil {
		t.Fatal(err)
	}
	p.fillBudget(t, 14)

	// A network retry of a like that already went through is not a new like.
	if _, err := p.swipe(t, p.candidate, Right, "first"); err != nil {
		t.Errorf("retrying the saved like: %v", err)
	}

	other := hiring.Card{EmployerUserID: p.employer.ID, Status: hiring.StatusLive, Title: "Go", Skills: []string{}, PayCurrency: "USD"}
	p.db.Create(&other)
	var limited *selectivity.Limited
	_, err := Record(p.ctx, p.db, p.candidate, other.ID, p.candidate.ID, Right, "over", 5000)
	if !errors.As(err, &limited) {
		t.Fatalf("the 16th like: error = %v, want *selectivity.Limited", err)
	}
	var saved int64
	p.db.Model(&Swipe{}).Where("hiring_card_id = ?", other.ID).Count(&saved)
	if saved != 0 {
		t.Error("the refused like was saved")
	}
	if _, err := Record(p.ctx, p.db, p.candidate, other.ID, p.candidate.ID, Left, "skip", 5000); err != nil {
		t.Errorf("skipping with no likes left: %v", err)
	}
}

// Several tabs take the last like at the same moment: exactly one gets it.
// Uses real transactions, so it removes its own rows.
func TestTheLastLikeGoesToOneTab(t *testing.T) {
	db := dbtest.Conn(t)
	cleanup := func() { removeUsers(db, "swipes._.budget.%@example.com") }
	cleanup()
	t.Cleanup(cleanup)

	for round := range 5 {
		p := newPair(t, db, fmt.Sprintf("budget.%d", round))
		p.fillBudget(t, 14)
		const tabs = 6
		cards := make([]string, tabs)
		for i := range cards {
			card := hiring.Card{EmployerUserID: p.employer.ID, Status: hiring.StatusLive, Title: "Go", Skills: []string{}, PayCurrency: "USD"}
			if err := db.Create(&card).Error; err != nil {
				t.Fatal(err)
			}
			cards[i] = card.ID
		}
		var wg sync.WaitGroup
		errs := make([]error, tabs)
		for i := range cards {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = Record(p.ctx, db, p.candidate, cards[i], p.candidate.ID, Right, fmt.Sprint("tab-", i), 5000)
			}()
		}
		wg.Wait()

		accepted := 0
		for _, err := range errs {
			var limited *selectivity.Limited
			switch {
			case err == nil:
				accepted++
			case !errors.As(err, &limited):
				t.Fatalf("round %d: %v", round, err)
			}
		}
		if accepted != 1 {
			t.Errorf("round %d: %d tabs got the last like, want 1", round, accepted)
		}
	}
}
