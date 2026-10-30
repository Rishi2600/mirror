package httpserver

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"testing"
	"time"

	"mirror/server/internal/hiring"
	"mirror/server/internal/matching"
	"mirror/server/internal/users"
)

func TestTimeZoneIsSavedAtSignIn(t *testing.T) {
	c := newAuthClient(t)
	cases := []struct{ email, sent, want string }{
		{"zone.good@example.com", "Asia/Kolkata", "Asia/Kolkata"},
		{"zone.bad@example.com", "Not/AZone", ""},
		{"zone.none@example.com", "", ""},
	}
	for _, tc := range cases {
		c.do("POST", "/v1/auth/otp/request", map[string]string{"email": tc.email}, nil)
		code := regexp.MustCompile(`\d{6}`).FindString(c.mailer.body)
		res := c.do("POST", "/v1/auth/otp/verify", map[string]string{"email": tc.email, "code": code, "time_zone": tc.sent}, nil)
		if res.StatusCode != http.StatusOK {
			t.Errorf("%s: sign-in status %d", tc.email, res.StatusCode)
		}
		var zone string
		c.tx.Raw("SELECT time_zone FROM users WHERE email = ?", tc.email).Scan(&zone)
		if zone != tc.want {
			t.Errorf("%s: time zone %q, want %q", tc.email, zone, tc.want)
		}
	}
}

func TestLikesOverTheAPI(t *testing.T) {
	ctx := context.Background()
	employer := newAuthClient(t)
	employer.signInAs("likes.hirer@example.com", "employer")
	employer.do("PUT", "/v1/me", map[string]string{"first_name": "Ana"}, nil)
	var card HiringCardResponse
	employer.do("POST", "/v1/hiring-cards", map[string]string{"description": jobDescription}, &card)
	employer.do("PUT", "/v1/hiring-cards/"+card.ID+"/status", map[string]string{"status": "live"}, nil)
	if err := matching.EmbedOwner(ctx, employer.tx, sameEmbedder{}, matching.OwnerHiringCard, card.ID); err != nil {
		t.Fatal(err)
	}
	candidate := *employer
	candidate.http = &http.Client{Jar: newJar()}
	candidateID := finishedCandidate(t, &candidate, "likes.seeker@example.com")

	var status SelectivityResponse
	candidate.do("GET", "/v1/selectivity", nil, &status)
	if status.Likes == nil || status.Likes.Left != 15 || status.Likes.DailyBudget != 15 || status.Likes.PausedUntil != nil {
		t.Fatalf("a new candidate's likes = %+v", status.Likes)
	}
	if body := body(t, &candidate, "/v1/selectivity"); regexp.MustCompile(`"score"`).MatchString(body) {
		t.Errorf("the raw score is exposed: %s", body)
	}

	// 15 likes already sent today.
	employerID := ""
	employer.tx.Raw("SELECT id FROM users WHERE email = 'likes.hirer@example.com'").Scan(&employerID)
	for range 15 {
		other := hiring.Card{EmployerUserID: employerID, Status: hiring.StatusLive, Title: "Go", Skills: []string{}, PayCurrency: "USD"}
		employer.tx.Create(&other)
		employer.tx.Exec(`INSERT INTO swipes (side, actor_user_id, hiring_card_id, candidate_user_id, direction, idempotency_key, dwell_ms)
			VALUES ('candidate', ?, ?, ?, 'right', gen_random_uuid()::text, 5000)`, candidateID, other.ID, candidateID)
	}

	// A like is refused with 429, the end time and Retry-After.
	res := candidate.swipe(map[string]string{"hiring_card_id": card.ID, "direction": "right"}, "over", nil)
	if res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("over budget: status %d, want 429", res.StatusCode)
	}
	retry, err := strconv.Atoi(res.Header.Get("Retry-After"))
	if err != nil || retry <= 0 || retry > 25*3600 {
		t.Errorf("Retry-After = %q", res.Header.Get("Retry-After"))
	}
	candidate.do("GET", "/v1/selectivity", nil, &status)
	if status.Likes.Left != 0 {
		t.Errorf("likes left = %d, want 0", status.Likes.Left)
	}

	// Skips still work, and the dwell time and fit are saved with the swipe.
	var saved struct {
		DwellMS *int `gorm:"column:dwell_ms"`
		Fit     *float64
	}
	dwell := map[string]any{"hiring_card_id": card.ID, "direction": "left", "dwell_ms": 4200}
	if res := candidate.swipeBody(dwell, "skip"); res.StatusCode != http.StatusOK {
		t.Fatalf("a skip with no likes left: status %d", res.StatusCode)
	}
	employer.tx.Raw("SELECT dwell_ms, fit FROM swipes WHERE hiring_card_id = ?", card.ID).Scan(&saved)
	if saved.DwellMS == nil || *saved.DwellMS != 4200 || saved.Fit == nil {
		t.Errorf("saved swipe = %+v, want the dwell time and a fit", saved)
	}

	// An employer's status has no likes, and flags ignored matches.
	var mine SelectivityResponse
	employer.do("GET", "/v1/selectivity", nil, &mine)
	if mine.Likes != nil || mine.Ghosting {
		t.Errorf("employer status = %+v", mine)
	}
	for i := range 3 {
		other := hiring.Card{EmployerUserID: employerID, Status: hiring.StatusLive, Title: "Go", Skills: []string{}, PayCurrency: "USD"}
		employer.tx.Create(&other)
		who, _ := users.Create(ctx, employer.tx, "likes.ignored."+strconv.Itoa(i)+"@example.com", users.RoleCandidate)
		employer.tx.Exec(`INSERT INTO matches (hiring_card_id, candidate_user_id, employer_user_id, matched_at) VALUES (?, ?, ?, ?)`,
			other.ID, who.ID, employerID, time.Now().Add(-5*24*time.Hour))
	}
	employer.do("GET", "/v1/selectivity", nil, &mine)
	if !mine.Ghosting {
		t.Error("an employer ignoring 3 matches should be flagged")
	}
}
