package matching

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"mirror/server/internal/dbtest"
	"mirror/server/internal/embed"
	"mirror/server/internal/hiring"
	"mirror/server/internal/profiles"
	"mirror/server/internal/users"
)

// topicEmbedder gives each text a vector from the topics it mentions, so
// tests control which cards are close. It counts its calls.
type topicEmbedder struct{ calls int }

var topics = []string{"go", "design", "data"}

func (e *topicEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	e.calls++
	out := make([][]float32, len(texts))
	for i, text := range texts {
		v := make([]float32, embed.Dims)
		v[len(topics)] = 0.2 // a shared base, so no two cards are perpendicular
		for j, topic := range topics {
			if strings.Contains(strings.ToLower(text), topic) {
				v[j] = 1
			}
		}
		var norm float64
		for _, x := range v {
			norm += float64(x * x)
		}
		for k := range v {
			v[k] /= float32(math.Sqrt(norm))
		}
		out[i] = v
	}
	return out, nil
}

type world struct {
	ctx      context.Context
	tx       *gorm.DB
	embedder *topicEmbedder
	employer string
}

func newWorld(t *testing.T) world {
	t.Helper()
	ctx := context.Background()
	tx := dbtest.Tx(t)
	u, _ := users.Create(ctx, tx, "employer@example.com", users.RoleEmployer)
	users.SetFirstName(ctx, tx, u.ID, "Ana")
	return world{ctx: ctx, tx: tx, embedder: &topicEmbedder{}, employer: u.ID}
}

// candidate adds a finished candidate card and embeds it.
func (w world) candidate(t *testing.T, email string, edit func(*profiles.Profile)) profiles.Profile {
	t.Helper()
	u, err := users.Create(w.ctx, w.tx, email, users.RoleCandidate)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	p := profiles.Profile{
		UserID: u.ID, ParseStatus: profiles.StatusDone, OnboardedAt: &now,
		CV:       profiles.CV{FullName: "Test Person", Headline: "Go backend engineer", Seniority: "senior", Skills: []string{"Go", "PostgreSQL"}},
		WorkMode: "any", PayCurrency: "EUR", EmployerDisplay: "name", ShowPay: true,
		RoleTypes: []string{}, Locations: []string{}, HiddenFrom: []string{},
	}
	if edit != nil {
		edit(&p)
	}
	if err := w.tx.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	if err := EmbedOwner(w.ctx, w.tx, w.embedder, OwnerCandidate, p.UserID); err != nil {
		t.Fatal(err)
	}
	return p
}

// card adds a live hiring card from w.employer and embeds it.
func (w world) card(t *testing.T, title string, edit func(*hiring.Card)) hiring.Card {
	t.Helper()
	c := hiring.Card{
		EmployerUserID: w.employer, Status: hiring.StatusLive, Title: title,
		Summary: title, WorkMode: "remote", Seniority: "senior", Skills: []string{"Go"}, PayCurrency: "EUR",
	}
	if edit != nil {
		edit(&c)
	}
	if err := w.tx.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	if err := EmbedOwner(w.ctx, w.tx, w.embedder, OwnerHiringCard, c.ID); err != nil {
		t.Fatal(err)
	}
	return c
}

// titles lists this test employer's cards in a timeline; the dev database
// may hold other cards (for example from make seed).
func (w world) titles(matches []JobMatch) string {
	var out []string
	for _, m := range matches {
		if m.Card.EmployerUserID == w.employer {
			out = append(out, m.Card.Title)
		}
	}
	return strings.Join(out, ", ")
}

func TestCandidateTimelineFiltersAndOrders(t *testing.T) {
	w := newWorld(t)
	me := w.candidate(t, "me@example.com", func(p *profiles.Profile) {
		p.WorkMode, p.MinPay, p.HiddenFrom = "remote", 80000, []string{" Current Co "}
	})

	w.card(t, "Go backend engineer", nil)
	w.card(t, "Product design lead", func(c *hiring.Card) { c.Skills = []string{"Figma"} })
	w.card(t, "Go draft", func(c *hiring.Card) { c.Status = hiring.StatusDraft })
	w.card(t, "Go at my employer", func(c *hiring.Card) { c.CompanyName = "current co" })
	w.card(t, "Go underpaid", func(c *hiring.Card) { c.PayMin, c.PayMax = 40000, 50000 })
	w.card(t, "Go onsite", func(c *hiring.Card) { c.WorkMode = "onsite" })
	w.card(t, "Go intern", func(c *hiring.Card) { c.Seniority = "intern" })

	matches, err := CandidateTimeline(w.ctx, w.tx, me)
	if err != nil {
		t.Fatalf("CandidateTimeline: %v", err)
	}
	if got := w.titles(matches); got != "Go backend engineer, Product design lead" {
		t.Errorf("timeline = %q; want the Go card first and the filtered cards gone", got)
	}
	if len(Reasons(matches[0].Scores)) == 0 {
		t.Errorf("the best card has no reasons: %+v", matches[0].Scores)
	}
}

func TestVisaFilter(t *testing.T) {
	w := newWorld(t)
	me := w.candidate(t, "visa@example.com", func(p *profiles.Profile) { p.NeedsVisa = true })
	w.card(t, "Go no visa", nil)
	w.card(t, "Go with visa", func(c *hiring.Card) { c.VisaSponsorship = true })

	matches, _ := CandidateTimeline(w.ctx, w.tx, me)
	if got := w.titles(matches); got != "Go with visa" {
		t.Errorf("timeline = %q, want only the card that sponsors visas", got)
	}
}

func TestHiringCardTimeline(t *testing.T) {
	w := newWorld(t)
	card := w.card(t, "Go backend engineer", func(c *hiring.Card) { c.CompanyName = "Fictional Co" })

	ours := map[string]bool{}
	for _, c := range []struct {
		email string
		edit  func(*profiles.Profile)
	}{
		{"go@example.com", nil},
		{"designer@example.com", func(p *profiles.Profile) { p.CV.Headline, p.CV.Skills = "Product design lead", []string{"Figma"} }},
		{"hides@example.com", func(p *profiles.Profile) { p.HiddenFrom = []string{"FICTIONAL CO"} }},
		{"unfinished@example.com", func(p *profiles.Profile) { p.OnboardedAt = nil }},
	} {
		ours[w.candidate(t, c.email, c.edit).UserID] = true
	}

	matches, err := HiringCardTimeline(w.ctx, w.tx, card)
	if err != nil {
		t.Fatalf("HiringCardTimeline: %v", err)
	}
	var headlines []string
	for _, m := range matches {
		if ours[m.Profile.UserID] {
			headlines = append(headlines, m.Profile.CV.Headline)
		}
	}
	if got := strings.Join(headlines, ", "); got != "Go backend engineer, Product design lead" {
		t.Errorf("timeline = %q; want the Go candidate first, no hidden or unfinished ones", got)
	}
}

func TestNoVectorYet(t *testing.T) {
	w := newWorld(t)
	c := hiring.Card{EmployerUserID: w.employer, Status: hiring.StatusLive, Title: "Go", Skills: []string{}, PayCurrency: "USD"}
	if err := w.tx.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := HiringCardTimeline(w.ctx, w.tx, c); err != ErrNoVector {
		t.Errorf("error = %v, want ErrNoVector", err)
	}
}

func TestEmbedOwnerSkipsUnchangedText(t *testing.T) {
	w := newWorld(t)
	c := w.card(t, "Go backend engineer", nil)
	before := w.embedder.calls

	EmbedOwner(w.ctx, w.tx, w.embedder, OwnerHiringCard, c.ID)
	if w.embedder.calls != before {
		t.Error("unchanged text was embedded again")
	}
	w.tx.Model(&c).Update("summary", "Go backend engineer for payments")
	EmbedOwner(w.ctx, w.tx, w.embedder, OwnerHiringCard, c.ID)
	if w.embedder.calls != before+1 {
		t.Error("changed text was not embedded again")
	}
}

func TestMissingEmbeddings(t *testing.T) {
	w := newWorld(t)
	withVector := w.card(t, "Go with a vector", nil)
	bare := hiring.Card{EmployerUserID: w.employer, Status: hiring.StatusLive, Title: "No vector", Skills: []string{}, PayCurrency: "USD"}
	if err := w.tx.Create(&bare).Error; err != nil {
		t.Fatal(err)
	}

	// The dev database may hold other cards, so check only these two.
	missing, err := MissingEmbeddings(w.ctx, w.tx)
	listed := map[string]bool{}
	for _, o := range missing {
		listed[o.ID] = true
	}
	if err != nil || !listed[bare.ID] || listed[withVector.ID] {
		t.Errorf("MissingEmbeddings = %+v, %v; want the bare card listed and the embedded one not", missing, err)
	}
}

func TestSpreadEmployers(t *testing.T) {
	m := func(employer string) JobMatch { return JobMatch{Card: hiring.Card{EmployerUserID: employer}} }
	in := []JobMatch{m("a"), m("a"), m("a"), m("a"), m("b"), m("a")}
	var got []string
	for _, x := range spreadEmployers(in) {
		got = append(got, x.Card.EmployerUserID)
	}
	if strings.Join(got, "") != "aaabaa" {
		t.Errorf("order = %v, want no more than 3 from one employer in a row while others wait", got)
	}
}
