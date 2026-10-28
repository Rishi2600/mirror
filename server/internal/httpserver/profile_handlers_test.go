package httpserver

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"os"
	"testing"

	"mirror/server/internal/profiles"
)

// parsedCV stands in for the AI in these tests.
type parsedCV struct{}

func (parsedCV) ParseCV(ctx context.Context, text string) (profiles.CV, error) {
	return profiles.CV{FullName: "Jordan Example", Seniority: "senior", Skills: []string{"Go"}}, nil
}

func TestProfileFlow(t *testing.T) {
	c := newAuthClient(t)
	c.signInAs("profile@example.com", "candidate")

	if res := c.do("GET", "/v1/profile", nil, nil); res.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /v1/profile before upload: status %d, want 404", res.StatusCode)
	}

	cv, _ := os.ReadFile("../cvtext/testdata/sample-cv.pdf")
	var p ProfileResponse
	res := c.upload("/v1/profile/cv", "cv", cv, &p)
	if res.StatusCode != http.StatusAccepted || p.ParseStatus != "pending" || len(c.queue.fileIDs) != 1 {
		t.Fatalf("upload: status %d, profile %+v, jobs %v", res.StatusCode, p, c.queue.fileIDs)
	}

	edit := map[string]any{
		"cv":            map[string]any{"full_name": "Jordan E."},
		"preferences":   map[string]any{"work_mode": "remote", "min_pay": 90000, "pay_currency": "EUR", "role_types": []string{"Backend"}, "locations": []string{}},
		"card_settings": map[string]any{"employer_display": "name", "show_pay": true},
	}
	if res := c.do("PUT", "/v1/profile", edit, nil); res.StatusCode != http.StatusConflict {
		t.Errorf("edit while parsing: status %d, want 409", res.StatusCode)
	}

	// Run the queued job's work with the stand-in AI.
	var userID string
	c.tx.Raw("SELECT id FROM users WHERE email = ?", "profile@example.com").Scan(&userID)
	if err := profiles.ParseCV(context.Background(), c.tx, c.files, parsedCV{}, userID, c.queue.fileIDs[0]); err != nil {
		t.Fatalf("ParseCV: %v", err)
	}
	if c.do("GET", "/v1/profile", nil, &p); p.ParseStatus != "done" || p.CV.FullName != "Jordan Example" {
		t.Fatalf("profile after parsing = %+v", p)
	}

	if res := c.do("PUT", "/v1/profile", edit, &p); res.StatusCode != http.StatusOK || p.CV.FullName != "Jordan E." || p.Preferences.WorkMode != "remote" {
		t.Errorf("edit: status %d, profile %+v", res.StatusCode, p)
	}

	edit["preferences"].(map[string]any)["work_mode"] = "moon"
	if res := c.do("PUT", "/v1/profile", edit, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("bad work mode: status %d, want 400", res.StatusCode)
	}
}

func TestUploadRejectsBadFiles(t *testing.T) {
	c := newAuthClient(t)
	c.signInAs("files@example.com", "candidate")

	if res := c.upload("/v1/profile/cv", "cv", []byte("plain text, not a CV file"), nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("text file: status %d, want 400", res.StatusCode)
	}
	if res := c.upload("/v1/profile/cv", "resume", []byte("%PDF-1.4"), nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("wrong field name: status %d, want 400", res.StatusCode)
	}
	tooBig := append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("x"), maxCVBytes)...)
	if res := c.upload("/v1/profile/cv", "cv", tooBig, nil); res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("6 MB file: status %d, want 413", res.StatusCode)
	}
	if len(c.queue.fileIDs) != 0 {
		t.Errorf("bad uploads queued jobs: %v", c.queue.fileIDs)
	}
}

func TestProfileIsForCandidatesOnly(t *testing.T) {
	c := newAuthClient(t)
	c.signInAs("hiring@example.com", "employer")

	if res := c.do("GET", "/v1/profile", nil, nil); res.StatusCode != http.StatusForbidden {
		t.Errorf("employer GET /v1/profile: status %d, want 403", res.StatusCode)
	}
}

func TestCardFlow(t *testing.T) {
	c := newAuthClient(t)
	c.signInAs("card@example.com", "candidate")
	cv, _ := os.ReadFile("../cvtext/testdata/sample-cv.pdf")
	c.upload("/v1/profile/cv", "cv", cv, nil)
	var userID string
	c.tx.Raw("SELECT id FROM users WHERE email = ?", "card@example.com").Scan(&userID)
	profiles.ParseCV(context.Background(), c.tx, c.files, cardCV{}, userID, c.queue.fileIDs[0])

	var preview CardResponse
	c.do("GET", "/v1/card", nil, &preview)
	if len(preview.Missing) != 1 || preview.Onboarded {
		t.Fatalf("preview before the photo = %+v, want only the photo missing", preview)
	}
	if res := c.do("POST", "/v1/card/finish", nil, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("finish without a photo: status %d, want 400", res.StatusCode)
	}

	var p ProfileResponse
	if res := c.upload("/v1/profile/photo", "photo", pngBytes(t), &p); res.StatusCode != http.StatusOK || p.PhotoURL == "" {
		t.Fatalf("photo upload: status %d, profile %+v", res.StatusCode, p)
	}
	res := c.do("GET", p.PhotoURL, nil, nil)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("GET %s: status %d, type %q", p.PhotoURL, res.StatusCode, res.Header.Get("Content-Type"))
	}

	edit := map[string]any{
		"cv":            p.CV,
		"preferences":   p.Preferences,
		"card_settings": map[string]any{"employer_display": "description", "employer_description": "a payments company", "show_pay": false},
	}
	if res := c.do("PUT", "/v1/profile", edit, nil); res.StatusCode != http.StatusOK {
		t.Fatalf("saving card settings: status %d", res.StatusCode)
	}
	c.do("GET", "/v1/card", nil, &preview)
	if preview.Card.CurrentEmployer != "a payments company" || preview.Card.Name != "Jordan Example" || preview.Removed.Emails != 1 {
		t.Errorf("card = %+v, removed %+v", preview.Card, preview.Removed)
	}

	if res := c.do("POST", "/v1/card/finish", nil, &p); res.StatusCode != http.StatusOK || !p.Onboarded {
		t.Errorf("finish: status %d, onboarded %v", res.StatusCode, p.Onboarded)
	}
}

func TestBadPhotoIsRejected(t *testing.T) {
	c := newAuthClient(t)
	c.signInAs("badphoto@example.com", "candidate")
	cv, _ := os.ReadFile("../cvtext/testdata/sample-cv.pdf")
	c.upload("/v1/profile/cv", "cv", cv, nil)

	if res := c.upload("/v1/profile/photo", "photo", cv, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("a PDF as photo: status %d, want 400", res.StatusCode)
	}
}

// cardCV stands in for the AI: its summary hides an email address.
type cardCV struct{}

func (cardCV) ParseCV(ctx context.Context, text string) (profiles.CV, error) {
	return profiles.CV{
		FullName: "Jordan Example", Headline: "Senior backend engineer",
		Summary:         "Backend engineer. Write to jordan.example@example.com.",
		CurrentEmployer: "Fictional Payments Ltd",
	}, nil
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	return buf.Bytes()
}
