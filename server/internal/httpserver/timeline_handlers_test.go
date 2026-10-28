package httpserver

import (
	"context"
	"net/http"
	"os"
	"testing"

	"mirror/server/internal/embed"
	"mirror/server/internal/matching"
	"mirror/server/internal/profiles"
)

// sameEmbedder gives every text the same vector, so every pair that passes
// the filters is a perfect meaning match.
type sameEmbedder struct{}

func (sameEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = make([]float32, embed.Dims)
		out[i][0] = 1
	}
	return out, nil
}

// finishedCandidate makes the signed-in candidate's card finished, with a
// vector, the quick way (the card flow itself is tested elsewhere).
func finishedCandidate(t *testing.T, c *authClient, email string) string {
	t.Helper()
	c.signInAs(email, "candidate")
	cv, _ := os.ReadFile("../cvtext/testdata/sample-cv.pdf")
	c.upload("/v1/profile/cv", "cv", cv, nil)
	var userID string
	c.tx.Raw("SELECT id FROM users WHERE email = ?", email).Scan(&userID)
	profiles.ParseCV(context.Background(), c.tx, c.files, cardCV{}, userID, c.queue.fileIDs[len(c.queue.fileIDs)-1])
	c.upload("/v1/profile/photo", "photo", pngBytes(t), nil)
	if res := c.do("POST", "/v1/card/finish", nil, nil); res.StatusCode != http.StatusOK {
		t.Fatalf("finish: status %d", res.StatusCode)
	}
	if err := matching.EmbedOwner(context.Background(), c.tx, sameEmbedder{}, matching.OwnerCandidate, userID); err != nil {
		t.Fatal(err)
	}
	return userID
}

func TestTimelines(t *testing.T) {
	employer := newAuthClient(t)
	employer.signInAs("timeline.hirer@example.com", "employer")
	employer.do("PUT", "/v1/me", map[string]string{"first_name": "Ana"}, nil)
	var card HiringCardResponse
	employer.do("POST", "/v1/hiring-cards", map[string]string{"description": jobDescription}, &card)
	path := "/v1/hiring-cards/" + card.ID

	if res := employer.do("GET", path+"/timeline", nil, nil); res.StatusCode != http.StatusConflict {
		t.Errorf("timeline of a draft card: status %d, want 409", res.StatusCode)
	}
	employer.do("PUT", path+"/status", map[string]string{"status": "live"}, nil)
	if got := employer.queue.embedded; len(got) != 1 || got[0] != "hiring_card:"+card.ID {
		t.Errorf("going live queued %v, want the card's vector", got)
	}
	matching.EmbedOwner(context.Background(), employer.tx, sameEmbedder{}, matching.OwnerHiringCard, card.ID)

	candidate := *employer
	candidate.http = &http.Client{Jar: newJar()}
	if res := candidate.do("GET", "/v1/timeline", nil, nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("signed-out timeline: status %d, want 401", res.StatusCode)
	}
	candidateID := finishedCandidate(t, &candidate, "timeline.seeker@example.com")

	// The card's company is the candidate's current employer, which reading
	// the CV put on their "hide me from" list.
	var jobs []TimelineJob
	candidate.do("GET", "/v1/timeline", nil, &jobs)
	for _, j := range jobs {
		if j.CardID == card.ID {
			t.Fatal("the card shows to a candidate who hides its company")
		}
	}
	edit := card.Fields
	edit.CompanyName = "Sample Logistics Co"
	employer.do("PUT", path, edit, nil)

	if res := candidate.do("GET", "/v1/timeline", nil, &jobs); res.StatusCode != http.StatusOK || len(jobs) == 0 || jobs[0].CardID != card.ID {
		t.Fatalf("candidate timeline: status %d, jobs %+v", res.StatusCode, jobs)
	}
	if jobs[0].View.EmployerFirstName != "Ana" || len(jobs[0].Reasons) == 0 || jobs[0].Score <= 0 {
		t.Errorf("first job = %+v", jobs[0])
	}

	var people []TimelineCandidate
	if res := employer.do("GET", path+"/timeline", nil, &people); res.StatusCode != http.StatusOK || len(people) == 0 {
		t.Fatalf("hiring card timeline: status %d, %+v", res.StatusCode, people)
	}
	var me TimelineCandidate
	for _, p := range people {
		if p.CandidateID == candidateID {
			me = p
		}
	}
	if me.Card.Name != "Jordan Example" || me.Card.PhotoURL == "" {
		t.Errorf("our candidate on the card's timeline = %+v", me)
	}
	if res := employer.do("GET", me.Card.PhotoURL, nil, nil); res.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("employer loading the candidate photo: status %d", res.StatusCode)
	}

	if res := candidate.do("GET", path+"/timeline", nil, nil); res.StatusCode != http.StatusForbidden {
		t.Errorf("candidate reading a hiring card's timeline: status %d, want 403", res.StatusCode)
	}
	if res := employer.do("GET", "/v1/timeline", nil, nil); res.StatusCode != http.StatusForbidden {
		t.Errorf("employer reading a candidate timeline: status %d, want 403", res.StatusCode)
	}
}

func TestTimelineNeedsAFinishedCard(t *testing.T) {
	c := newAuthClient(t)
	c.signInAs("not.finished@example.com", "candidate")
	if res := c.do("GET", "/v1/timeline", nil, nil); res.StatusCode != http.StatusConflict {
		t.Errorf("timeline before a card: status %d, want 409", res.StatusCode)
	}
}
