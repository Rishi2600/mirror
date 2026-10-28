package hiring

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"

	"gorm.io/gorm"

	"mirror/server/internal/dbtest"
	"mirror/server/internal/storage"
	"mirror/server/internal/users"
)

// fakeParser stands in for the AI, with messy values to clean.
type fakeParser struct{ err error }

func (f fakeParser) ParseJob(ctx context.Context, description string) (Fields, error) {
	return Fields{
		Title:       " Senior Backend Engineer ",
		CompanyName: "Fictional Payments Ltd",
		WorkMode:    "Remote",
		Seniority:   "Senior",
		Skills:      []string{"Go", "go", " PostgreSQL "},
		Summary:     "Build payment APIs. Apply at jobs@example.com.",
		PayMin:      90000,
		PayMax:      120000,
		PayCurrency: "eur",
	}, f.err
}

const description = "We are hiring a senior backend engineer to build payment APIs in Go and PostgreSQL. Remote within the EU."

func setup(t *testing.T) (context.Context, *gorm.DB, users.User) {
	t.Helper()
	ctx := context.Background()
	tx := dbtest.Tx(t)
	u, err := users.Create(ctx, tx, "employer@example.com", users.RoleEmployer)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, tx, u
}

func TestCreateFromDescription(t *testing.T) {
	ctx, tx, u := setup(t)

	c, err := Create(ctx, tx, fakeParser{}, u.ID, description)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Status != StatusDraft || c.Title != "Senior Backend Engineer" || c.WorkMode != "remote" ||
		c.Seniority != "senior" || c.PayCurrency != "EUR" || strings.Join(c.Skills, ",") != "Go,PostgreSQL" {
		t.Errorf("created card was not cleaned: %+v", c)
	}
	if c.Description != description {
		t.Error("the pasted description was not kept")
	}

	if _, err := Create(ctx, tx, fakeParser{}, u.ID, "too short"); !errors.Is(err, ErrDescription) {
		t.Errorf("short description: error = %v, want ErrDescription", err)
	}
	if _, err := Create(ctx, tx, fakeParser{err: errors.New("groq down")}, u.ID, description); err == nil {
		t.Error("Create hid the AI error")
	}
}

func TestEmployersOnlySeeTheirOwnCards(t *testing.T) {
	ctx, tx, u := setup(t)
	other, _ := users.Create(ctx, tx, "other@example.com", users.RoleEmployer)
	mine, _ := Create(ctx, tx, fakeParser{}, u.ID, description)
	Create(ctx, tx, fakeParser{}, u.ID, description)
	Create(ctx, tx, fakeParser{}, other.ID, description)

	if list, _ := List(ctx, tx, u.ID); len(list) != 2 {
		t.Errorf("List returned %d cards, want this employer's 2", len(list))
	}
	if _, err := Get(ctx, tx, other.ID, mine.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("another employer's Get: error = %v, want ErrNotFound", err)
	}
	if _, err := Update(ctx, tx, other.ID, mine.ID, mine.Fields()); !errors.Is(err, ErrNotFound) {
		t.Errorf("another employer's Update: error = %v, want ErrNotFound", err)
	}
	if _, err := SetStatus(ctx, tx, other.ID, mine.ID, StatusClosed, "Ana"); !errors.Is(err, ErrNotFound) {
		t.Errorf("another employer's SetStatus: error = %v, want ErrNotFound", err)
	}
	if _, err := Get(ctx, tx, u.ID, "not-a-uuid"); !errors.Is(err, ErrNotFound) {
		t.Errorf("bad ID: error = %v, want ErrNotFound", err)
	}
}

func TestGoLiveAndClose(t *testing.T) {
	ctx, tx, u := setup(t)
	c, _ := Create(ctx, tx, fakeParser{}, u.ID, description)

	var missing MissingError
	if _, err := SetStatus(ctx, tx, u.ID, c.ID, StatusLive, ""); !errors.As(err, &missing) || len(missing.Missing) != 1 {
		t.Fatalf("going live without a first name: error = %v, want MissingError", err)
	}
	if c, _ = SetStatus(ctx, tx, u.ID, c.ID, StatusLive, "Ana"); c.Status != StatusLive {
		t.Fatalf("status = %q, want live", c.Status)
	}
	if c, _ = SetStatus(ctx, tx, u.ID, c.ID, StatusClosed, "Ana"); c.Status != StatusClosed {
		t.Errorf("status = %q, want closed", c.Status)
	}
	if _, err := SetStatus(ctx, tx, u.ID, c.ID, StatusDraft, "Ana"); !errors.Is(err, ErrStatus) {
		t.Errorf("back to draft: error = %v, want ErrStatus", err)
	}
}

func TestUpdateAndValidate(t *testing.T) {
	ctx, tx, u := setup(t)
	c, _ := Create(ctx, tx, fakeParser{}, u.ID, description)

	f := c.Fields()
	f.Title, f.Location, f.WorkMode = "Staff Engineer", "Berlin", "hybrid"
	if c, _ = Update(ctx, tx, u.ID, c.ID, f); c.Title != "Staff Engineer" || c.Location != "Berlin" || c.WorkMode != "hybrid" {
		t.Errorf("after Update: %+v", c)
	}

	for _, bad := range []Fields{
		{WorkMode: "moon", PayCurrency: "USD"},
		{Seniority: "wizard", PayCurrency: "USD"},
		{PayMin: 100, PayMax: 50, PayCurrency: "USD"},
		{PayCurrency: "us"},
	} {
		if err := Validate(bad); err == nil {
			t.Errorf("Validate accepted %+v", bad)
		}
	}
}

func TestViewHidesContactDetails(t *testing.T) {
	ctx, tx, u := setup(t)
	c, _ := Create(ctx, tx, fakeParser{}, u.ID, description)

	view, removed := BuildView(c, "Ana", "")
	if strings.Contains(view.Summary, "@") || removed.Emails != 1 {
		t.Errorf("summary %q, removed %+v; want the email gone", view.Summary, removed)
	}
	if view.EmployerFirstName != "Ana" || view.CompanyName != "Fictional Payments Ltd" {
		t.Errorf("view = %+v", view)
	}
}

func TestImage(t *testing.T) {
	ctx, tx, u := setup(t)
	files, _ := storage.NewLocal(t.TempDir(), bytes.Repeat([]byte{1}, 32))
	c, _ := Create(ctx, tx, fakeParser{}, u.ID, description)

	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	c, err := SetImage(ctx, tx, files, u.ID, c.ID, buf.Bytes())
	if err != nil || c.ImageFileID == nil {
		t.Fatalf("SetImage = %+v, %v", c, err)
	}
	if jpg, err := Image(ctx, tx, files, c); err != nil || !bytes.HasPrefix(jpg, []byte{0xFF, 0xD8}) {
		t.Errorf("Image = %d bytes, %v; want a JPEG", len(jpg), err)
	}
}
