package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// swipe posts a swipe with an Idempotency-Key header.
func (c *authClient) swipe(body map[string]string, key string, out any) *http.Response {
	c.t.Helper()
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", c.base+"/v1/swipes", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil {
		json.NewDecoder(res.Body).Decode(out)
	}
	return res
}

func body(t *testing.T, c *authClient, path string) string {
	t.Helper()
	res, err := c.http.Get(c.base + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return string(data)
}

// The done check for chunk 9: two users match, and only then does the
// employer see the candidate's contact details.
func TestMatchRevealsOnlyAfterBothLike(t *testing.T) {
	employer := newAuthClient(t)
	employer.signInAs("match.hirer@example.com", "employer")
	employer.do("PUT", "/v1/me", map[string]string{"first_name": "Ana"}, nil)
	var card HiringCardResponse
	employer.do("POST", "/v1/hiring-cards", map[string]string{"description": jobDescription}, &card)
	edit := card.Fields
	edit.CompanyName = "Sample Logistics Co"
	employer.do("PUT", "/v1/hiring-cards/"+card.ID, edit, nil)
	employer.do("PUT", "/v1/hiring-cards/"+card.ID+"/status", map[string]string{"status": "live"}, nil)

	candidate := *employer
	candidate.http = &http.Client{Jar: newJar()}
	candidateID := finishedCandidate(t, &candidate, "match.seeker@example.com")
	like := map[string]string{"hiring_card_id": card.ID, "direction": "right"}

	if res := candidate.swipe(like, "", nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("swipe without a key: status %d, want 400", res.StatusCode)
	}
	var first, retry SwipeResponse
	if res := candidate.swipe(like, "cand-1", &first); res.StatusCode != http.StatusOK || first.Matched {
		t.Fatalf("candidate like: status %d, %+v; want no match yet", res.StatusCode, first)
	}
	if res := candidate.swipe(like, "cand-1", &retry); res.StatusCode != http.StatusOK || retry != first {
		t.Errorf("retried swipe: status %d, %+v; want the same answer", res.StatusCode, retry)
	}
	if res := candidate.swipe(like, "cand-2", nil); res.StatusCode != http.StatusConflict {
		t.Errorf("second swipe on the pair: status %d, want 409", res.StatusCode)
	}

	// One-sided: nothing to see yet, and no contact details anywhere.
	var none []MatchResponse
	if employer.do("GET", "/v1/matches", nil, &none); len(none) != 0 {
		t.Errorf("employer has %d matches before liking back, want 0", len(none))
	}
	if strings.Contains(body(t, employer, "/v1/hiring-cards/"+card.ID+"/timeline"), "match.seeker@example.com") {
		t.Error("the candidate's email shows on the timeline before a match")
	}

	var made SwipeResponse
	employer.swipe(map[string]string{"hiring_card_id": card.ID, "candidate_id": candidateID, "direction": "right"}, "emp-1", &made)
	if !made.Matched || made.MatchID == "" {
		t.Fatalf("employer like back: %+v, want a match", made)
	}

	var revealed MatchResponse
	employer.do("GET", "/v1/matches/"+made.MatchID, nil, &revealed)
	if revealed.Contact == nil || revealed.Contact.Email != "match.seeker@example.com" || revealed.Contact.FullName != "Jordan Example" {
		t.Fatalf("employer's match page = %+v, want the contact details", revealed)
	}
	if revealed.Contact.CVURL == "" {
		t.Fatal("no CV link")
	}
	res, _ := employer.http.Get(employer.base + revealed.Contact.CVURL)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/pdf" {
		t.Errorf("CV download: status %d, type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	tampered := strings.Replace(revealed.Contact.CVURL, "exp=", "exp=9", 1)
	if res, _ := employer.http.Get(employer.base + tampered); res.StatusCode != http.StatusForbidden {
		t.Errorf("CV link with a changed expiry: status %d, want 403", res.StatusCode)
	}
	if res, _ := candidate.http.Get(candidate.base + revealed.Contact.CVURL); res.StatusCode != http.StatusForbidden {
		t.Errorf("the candidate opening the employer's CV link: status %d, want 403", res.StatusCode)
	}

	// The candidate sees the job, and no contact details of anyone.
	page := body(t, &candidate, "/v1/matches/"+made.MatchID)
	if !strings.Contains(page, `"job"`) || strings.Contains(page, `"contact"`) {
		t.Errorf("candidate's match page = %s", page)
	}

	if res := candidate.do("DELETE", "/v1/matches/"+made.MatchID, nil, nil); res.StatusCode != http.StatusNoContent {
		t.Fatalf("unmatch: status %d", res.StatusCode)
	}
	if res := employer.do("GET", "/v1/matches/"+made.MatchID, nil, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("employer after unmatch: status %d, want 404", res.StatusCode)
	}
	if res, _ := employer.http.Get(employer.base + revealed.Contact.CVURL); res.StatusCode != http.StatusNotFound {
		t.Errorf("CV link after unmatch: status %d, want 404", res.StatusCode)
	}

	var audits []string
	employer.tx.Raw("SELECT action FROM audit_events WHERE match_id = ? ORDER BY id", made.MatchID).Scan(&audits)
	if strings.Join(audits, ",") != "reveal,cv_download,unmatch" {
		t.Errorf("audit trail = %v, want reveal, cv_download, unmatch", audits)
	}
}
