package profiles

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"strings"
	"testing"

	"gorm.io/gorm"

	"mirror/server/internal/dbtest"
	"mirror/server/internal/storage"
	"mirror/server/internal/users"
)

// fakeParser returns a fixed CV and records the text it was given.
type fakeParser struct {
	text string
	err  error
}

func (f *fakeParser) ParseCV(ctx context.Context, text string) (CV, error) {
	f.text = text
	return CV{
		FullName:        "  Jordan Example ",
		Headline:        "Senior backend engineer",
		Seniority:       "Senior",
		CurrentEmployer: "Fictional Payments Ltd",
		Skills:          []string{"Go", "go", " PostgreSQL ", ""},
	}, f.err
}

type env struct {
	ctx   context.Context
	tx    *gorm.DB
	files *storage.Local
	user  string
}

func setup(t *testing.T) env {
	t.Helper()
	ctx := context.Background()
	tx := dbtest.Tx(t)
	files, err := storage.NewLocal(t.TempDir(), bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	u, err := users.Create(ctx, tx, "candidate@example.com", users.RoleCandidate)
	if err != nil {
		t.Fatal(err)
	}
	return env{ctx, tx, files, u.ID}
}

func sampleCV(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../cvtext/testdata/sample-cv.pdf")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestUploadThenParse(t *testing.T) {
	e := setup(t)

	p, fileID, err := UploadCV(e.ctx, e.tx, e.files, e.user, sampleCV(t))
	if err != nil {
		t.Fatalf("UploadCV: %v", err)
	}
	if p.ParseStatus != StatusPending || p.WorkMode != "any" || p.PayCurrency != "USD" {
		t.Errorf("new profile = %+v, want pending with default preferences", p)
	}

	parser := &fakeParser{}
	if err := ParseCV(e.ctx, e.tx, e.files, parser, e.user, fileID); err != nil {
		t.Fatalf("ParseCV: %v", err)
	}
	if !strings.Contains(parser.text, "Jordan Example") {
		t.Errorf("the parser did not get the CV text: %q", parser.text)
	}

	p, _ = Get(e.ctx, e.tx, e.user)
	if p.ParseStatus != StatusDone {
		t.Fatalf("status = %q, want done", p.ParseStatus)
	}
	if p.CV.FullName != "Jordan Example" || p.CV.Seniority != "senior" {
		t.Errorf("CV was not cleaned: %+v", p.CV)
	}
	if strings.Join(p.CV.Skills, ",") != "Go,PostgreSQL" {
		t.Errorf("skills = %q, want trimmed with repeats removed", p.CV.Skills)
	}
	if strings.Join(p.HiddenFrom, ",") != "Fictional Payments Ltd" {
		t.Errorf("hidden_from = %q, want the current employer", p.HiddenFrom)
	}
}

func TestReuploadReplacesTheOldFile(t *testing.T) {
	e := setup(t)
	_, firstID, _ := UploadCV(e.ctx, e.tx, e.files, e.user, sampleCV(t))
	_, secondID, err := UploadCV(e.ctx, e.tx, e.files, e.user, sampleCV(t))
	if err != nil {
		t.Fatal(err)
	}

	var count int64
	e.tx.Model(&File{}).Where("owner_user_id = ?", e.user).Count(&count)
	if count != 1 {
		t.Errorf("%d file rows, want only the new one", count)
	}

	// A job still running for the first file must not touch the profile.
	if err := ParseCV(e.ctx, e.tx, e.files, &fakeParser{}, e.user, firstID); err != nil {
		t.Fatalf("ParseCV for the old file: %v", err)
	}
	if p, _ := Get(e.ctx, e.tx, e.user); p.ParseStatus != StatusPending || *p.CVFileID != secondID {
		t.Errorf("profile = %+v, want still pending on the new file", p)
	}
}

func TestUnsupportedUploadIsRejected(t *testing.T) {
	e := setup(t)
	if _, _, err := UploadCV(e.ctx, e.tx, e.files, e.user, []byte("just text")); err == nil {
		t.Error("UploadCV accepted a plain text file")
	}
}

func TestCVWithoutTextFails(t *testing.T) {
	e := setup(t)
	_, fileID, err := UploadCV(e.ctx, e.tx, e.files, e.user, emptyDOCX(t))
	if err != nil {
		t.Fatal(err)
	}

	if err := ParseCV(e.ctx, e.tx, e.files, &fakeParser{}, e.user, fileID); err != nil {
		t.Fatalf("ParseCV returned %v; want nil (retrying cannot help)", err)
	}
	p, _ := Get(e.ctx, e.tx, e.user)
	if p.ParseStatus != StatusFailed || p.ParseError == "" {
		t.Errorf("profile = %+v, want failed with a message", p)
	}
}

func TestAIErrorIsReturnedForRetry(t *testing.T) {
	e := setup(t)
	_, fileID, _ := UploadCV(e.ctx, e.tx, e.files, e.user, sampleCV(t))

	err := ParseCV(e.ctx, e.tx, e.files, &fakeParser{err: errors.New("rate limited")}, e.user, fileID)
	if err == nil {
		t.Error("ParseCV hid the AI error")
	}
	if p, _ := Get(e.ctx, e.tx, e.user); p.ParseStatus != StatusPending {
		t.Errorf("status = %q, want still pending", p.ParseStatus)
	}
}

func TestUpdate(t *testing.T) {
	e := setup(t)
	_, fileID, _ := UploadCV(e.ctx, e.tx, e.files, e.user, sampleCV(t))
	prefs := Preferences{RoleTypes: []string{"Backend", " backend "}, WorkMode: "remote", MinPay: 90000, PayCurrency: "EUR", NeedsVisa: true}

	settings := CardSettings{EmployerDisplay: "name", ShowPay: true}
	if _, err := Update(e.ctx, e.tx, e.user, CV{FullName: "J"}, prefs, settings); !errors.Is(err, ErrParsing) {
		t.Errorf("Update while parsing: error = %v, want ErrParsing", err)
	}

	ParseCV(e.ctx, e.tx, e.files, &fakeParser{}, e.user, fileID)
	if _, err := Update(e.ctx, e.tx, e.user, CV{FullName: "Jordan E."}, prefs, settings); err != nil {
		t.Fatalf("Update: %v", err)
	}
	p, _ := Get(e.ctx, e.tx, e.user)
	if p.CV.FullName != "Jordan E." || p.WorkMode != "remote" || p.MinPay != 90000 || !p.NeedsVisa {
		t.Errorf("profile after Update = %+v", p)
	}
	if strings.Join(p.RoleTypes, ",") != "Backend" {
		t.Errorf("role types = %q, want repeats removed", p.RoleTypes)
	}
}

func TestValidatePreferences(t *testing.T) {
	good := Preferences{WorkMode: "hybrid", PayCurrency: "INR"}
	if err := ValidatePreferences(good); err != nil {
		t.Errorf("valid preferences rejected: %v", err)
	}
	for _, bad := range []Preferences{
		{WorkMode: "space", PayCurrency: "USD"},
		{WorkMode: "any", PayCurrency: "usd"},
		{WorkMode: "any", PayCurrency: "USD", MinPay: -1},
	} {
		if err := ValidatePreferences(bad); err == nil {
			t.Errorf("invalid preferences accepted: %+v", bad)
		}
	}
}

// emptyDOCX is a valid DOCX file with no text in it.
func emptyDOCX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	w, _ := z.Create("word/document.xml")
	w.Write([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p/></w:body></w:document>`))
	z.Close()
	return buf.Bytes()
}

func TestPhotoAndFinish(t *testing.T) {
	e := setup(t)
	_, fileID, _ := UploadCV(e.ctx, e.tx, e.files, e.user, sampleCV(t))
	ParseCV(e.ctx, e.tx, e.files, &fakeParser{}, e.user, fileID)

	var missing MissingError
	if _, err := Finish(e.ctx, e.tx, e.user); !errors.As(err, &missing) || len(missing.Missing) != 1 {
		t.Fatalf("Finish without a photo: error = %v, want MissingError for the photo", err)
	}

	if _, err := SetPhoto(e.ctx, e.tx, e.files, e.user, []byte("not an image")); err == nil {
		t.Error("SetPhoto accepted a non-image")
	}
	for range 2 { // the second photo replaces the first
		if _, err := SetPhoto(e.ctx, e.tx, e.files, e.user, tinyPNG(t)); err != nil {
			t.Fatalf("SetPhoto: %v", err)
		}
	}
	var photos int64
	e.tx.Model(&File{}).Where("owner_user_id = ? AND kind = 'photo'", e.user).Count(&photos)
	if photos != 1 {
		t.Errorf("%d photo files, want only the latest", photos)
	}
	if jpg, err := Photo(e.ctx, e.tx, e.files, e.user); err != nil || !bytes.HasPrefix(jpg, []byte{0xFF, 0xD8}) {
		t.Errorf("Photo = %d bytes, %v; want a JPEG", len(jpg), err)
	}

	if _, err := Finish(e.ctx, e.tx, e.user); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	p, _ := Get(e.ctx, e.tx, e.user)
	if p.OnboardedAt == nil {
		t.Fatal("onboarded_at is not set")
	}
	Finish(e.ctx, e.tx, e.user)
	if again, _ := Get(e.ctx, e.tx, e.user); !again.OnboardedAt.Equal(*p.OnboardedAt) {
		t.Error("finishing again changed onboarded_at")
	}
}

// tinyPNG is a 2x2 PNG image.
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
