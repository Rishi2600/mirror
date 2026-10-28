package httpserver

import (
	"context"
	"net/http"
	"testing"

	"mirror/server/internal/hiring"
)

// fakeJobParser stands in for the AI; its summary hides an email address.
type fakeJobParser struct{}

func (fakeJobParser) ParseJob(ctx context.Context, description string) (hiring.Fields, error) {
	return hiring.Fields{
		Title: "Senior Backend Engineer", CompanyName: "Fictional Payments Ltd",
		WorkMode: "remote", Seniority: "senior", Skills: []string{"Go"},
		Summary: "Build payment APIs. Questions: hiring@example.com", PayCurrency: "EUR",
	}, nil
}

const jobDescription = "We are hiring a senior backend engineer to build payment APIs in Go and PostgreSQL. Remote."

func TestHiringCardFlow(t *testing.T) {
	c := newAuthClient(t)
	c.signInAs("hirer@example.com", "employer")

	var card HiringCardResponse
	if res := c.do("POST", "/v1/hiring-cards", map[string]string{"description": jobDescription}, &card); res.StatusCode != http.StatusCreated || card.Status != "draft" || card.Title != "Senior Backend Engineer" {
		t.Fatalf("create: status %d, card %+v", res.StatusCode, card)
	}
	for range 2 {
		c.do("POST", "/v1/hiring-cards", map[string]string{"description": jobDescription}, nil)
	}
	var list []HiringCardResponse
	if c.do("GET", "/v1/hiring-cards", nil, &list); len(list) != 3 {
		t.Errorf("list has %d cards, want 3", len(list))
	}

	path := "/v1/hiring-cards/" + card.ID
	if res := c.do("PUT", path+"/status", map[string]string{"status": "live"}, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("live without a first name: status %d, want 400", res.StatusCode)
	}
	c.do("PUT", "/v1/me", map[string]string{"first_name": "Ana"}, nil)

	var preview HiringPreviewResponse
	c.do("GET", path+"/preview", nil, &preview)
	if preview.View.EmployerFirstName != "Ana" || len(preview.Missing) != 0 || preview.Removed.Emails != 1 {
		t.Errorf("preview = %+v", preview)
	}

	edit := card.Fields
	edit.Location = "Berlin"
	if res := c.do("PUT", path, edit, &card); res.StatusCode != http.StatusOK || card.Location != "Berlin" {
		t.Errorf("edit: status %d, card %+v", res.StatusCode, card)
	}
	edit.WorkMode = "moon"
	if res := c.do("PUT", path, edit, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("bad work mode: status %d, want 400", res.StatusCode)
	}

	if c.do("PUT", path+"/status", map[string]string{"status": "live"}, &card); card.Status != "live" {
		t.Errorf("status = %q, want live", card.Status)
	}
	if c.do("PUT", path+"/status", map[string]string{"status": "closed"}, &card); card.Status != "closed" {
		t.Errorf("status = %q, want closed", card.Status)
	}

	if res := c.upload(path+"/image", "image", pngBytes(t), &card); res.StatusCode != http.StatusOK || card.ImageURL == "" {
		t.Fatalf("image upload: status %d, card %+v", res.StatusCode, card)
	}
	if res := c.do("GET", card.ImageURL, nil, nil); res.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("GET image: status %d, type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
}

func TestHiringCardsAreTheEmployersOwn(t *testing.T) {
	owner := newAuthClient(t)
	owner.signInAs("owner@example.com", "employer")
	var card HiringCardResponse
	owner.do("POST", "/v1/hiring-cards", map[string]string{"description": jobDescription}, &card)

	// A second browser on the same server and database.
	other := *owner
	other.http = &http.Client{Jar: newJar()}
	other.signInAs("intruder@example.com", "employer")
	path := "/v1/hiring-cards/" + card.ID
	for _, req := range []struct{ method, path string }{{"GET", path}, {"PUT", path}, {"GET", path + "/preview"}, {"GET", path + "/image"}} {
		if res := other.do(req.method, req.path, card.Fields, nil); res.StatusCode != http.StatusNotFound {
			t.Errorf("%s %s by another employer: status %d, want 404", req.method, req.path, res.StatusCode)
		}
	}
	var list []HiringCardResponse
	if other.do("GET", "/v1/hiring-cards", nil, &list); len(list) != 0 {
		t.Errorf("another employer sees %d cards, want 0", len(list))
	}
}

func TestHiringCardsAreForEmployersOnly(t *testing.T) {
	c := newAuthClient(t)
	c.signInAs("job.seeker@example.com", "candidate")
	if res := c.do("GET", "/v1/hiring-cards", nil, nil); res.StatusCode != http.StatusForbidden {
		t.Errorf("candidate GET /v1/hiring-cards: status %d, want 403", res.StatusCode)
	}
	if res := c.do("POST", "/v1/hiring-cards", map[string]string{"description": jobDescription}, nil); res.StatusCode != http.StatusForbidden {
		t.Errorf("candidate POST /v1/hiring-cards: status %d, want 403", res.StatusCode)
	}
}
