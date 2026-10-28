// Command seed fills the dev database with made-up candidates and hiring
// cards, embeds them with the local model, and reports how well the
// timelines match them (paper, section 8). Run it with make seed.
//
// Every seeded person is fictional; their emails are seed.*@example.com,
// and running seed again replaces them.
package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"gorm.io/gorm"

	"mirror/server/internal/config"
	"mirror/server/internal/db"
	"mirror/server/internal/embed"
	"mirror/server/internal/hiring"
	"mirror/server/internal/matching"
	"mirror/server/internal/profiles"
	"mirror/server/internal/users"
)

type seedCandidate struct {
	field, name, headline, seniority string
	years                            int
	skills                           []string
	summary                          string
}

type seedCard struct {
	field, title, company, seniority, mode string
	skills                                 []string
	summary                                string
}

var candidates = []seedCandidate{
	{"backend", "Avery Test", "Senior backend engineer", "senior", 8, []string{"Go", "PostgreSQL", "Kubernetes", "gRPC"}, "Builds payment and order APIs in Go; owns Postgres schemas and on-call."},
	{"backend", "Blake Sample", "Backend developer", "mid", 4, []string{"Python", "Django", "PostgreSQL", "Redis"}, "Writes REST services in Python and Django for a logistics platform."},
	{"frontend", "Casey Example", "Frontend engineer", "mid", 5, []string{"React", "TypeScript", "CSS", "Vite"}, "Builds fast, accessible web apps in React and TypeScript."},
	{"frontend", "Drew Fiction", "Senior frontend developer", "senior", 9, []string{"Vue", "JavaScript", "Design systems"}, "Leads a component library and the checkout front end."},
	{"data", "Emery Mock", "Data analyst", "junior", 2, []string{"SQL", "Python", "Tableau"}, "Builds weekly sales dashboards and cleans messy data."},
	{"data", "Finley Dummy", "Machine learning engineer", "senior", 7, []string{"Python", "PyTorch", "MLOps"}, "Trains and ships recommendation models to production."},
	{"design", "Gray Placeholder", "Product designer", "mid", 5, []string{"Figma", "User research", "Prototyping"}, "Runs user research and designs mobile app flows."},
	{"design", "Harper Specimen", "UX designer", "senior", 10, []string{"Figma", "Design systems", "Accessibility"}, "Leads UX for a health app used by clinics."},
	{"devops", "Indigo Trial", "Site reliability engineer", "senior", 8, []string{"Kubernetes", "Terraform", "AWS", "Prometheus"}, "Keeps a busy platform up; automates infrastructure with Terraform."},
	{"devops", "Jules Demo", "DevOps engineer", "mid", 4, []string{"Docker", "CI/CD", "Linux", "Ansible"}, "Builds CI pipelines and manages Linux servers."},
	{"sales", "Kai Sample", "Account executive", "mid", 5, []string{"B2B sales", "CRM", "Negotiation"}, "Closes mid-market SaaS deals and grows accounts."},
	{"sales", "Lane Fixture", "Sales development rep", "junior", 1, []string{"Cold outreach", "CRM", "Prospecting"}, "Books meetings for the sales team through calls and email."},
	{"nursing", "Morgan Testcase", "Registered nurse", "mid", 6, []string{"Patient care", "ICU", "Medication administration"}, "Cares for patients in a busy intensive care unit."},
	{"nursing", "Noel Stub", "Pediatric nurse", "senior", 11, []string{"Pediatrics", "Patient care", "Triage"}, "Leads a pediatric ward team and trains new nurses."},
	{"cooking", "Oakley Mockup", "Sous chef", "mid", 6, []string{"French cuisine", "Kitchen management", "Menu planning"}, "Runs the line in a busy restaurant kitchen."},
	{"cooking", "Parker Sampleton", "Pastry chef", "senior", 9, []string{"Pastry", "Baking", "Desserts"}, "Creates dessert menus for a hotel group."},
}

var cards = []seedCard{
	{"backend", "Backend engineer (Go)", "Example Payments", "senior", "remote", []string{"Go", "PostgreSQL", "Kubernetes"}, "Build the payment events API in Go and own the database."},
	{"backend", "Backend-Entwickler (Python)", "Beispiel Logistik GmbH", "mid", "hybrid", []string{"Python", "Django", "PostgreSQL"}, "Entwickle REST-Dienste in Python für unsere Logistikplattform."},
	{"frontend", "Frontend engineer (React)", "Sample Shop", "mid", "remote", []string{"React", "TypeScript"}, "Build the storefront in React and TypeScript."},
	{"data", "Data analyst", "Example Retail", "junior", "remote", []string{"SQL", "Python", "Dashboards"}, "Build sales dashboards and present findings to store managers."},
	{"data", "ML engineer", "Fictional AI Labs", "senior", "hybrid", []string{"Python", "PyTorch"}, "Train and deploy ranking models."},
	{"design", "Diseñador de producto", "Ejemplo Salud", "mid", "remote", []string{"Figma", "Investigación de usuarios"}, "Diseña flujos para nuestra app de pacientes y realiza investigación con usuarios."},
	{"devops", "Platform engineer", "Sample Cloud", "senior", "remote", []string{"Kubernetes", "Terraform", "AWS"}, "Run our Kubernetes platform and infrastructure as code."},
	{"sales", "Account executive", "Example SaaS", "mid", "hybrid", []string{"B2B sales", "CRM"}, "Sell our product to mid-market companies."},
	{"nursing", "ICU nurse", "Example City Hospital", "mid", "onsite", []string{"Patient care", "ICU"}, "Care for critically ill patients on the night shift."},
	{"cooking", "Pastry chef", "Sample Hotels", "senior", "onsite", []string{"Pastry", "Desserts"}, "Lead the pastry section of our hotel kitchen."},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	conn, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close(conn)
	if _, err := db.Migrate(ctx, conn); err != nil {
		return err
	}
	embedder := embed.NewTEI(cfg.EmbedURL)
	if err := embedder.Check(ctx); err != nil {
		return fmt.Errorf("%w (start it with make db-up)", err)
	}

	if err := removeOldSeed(ctx, conn); err != nil {
		return err
	}
	field := map[string]string{} // card or candidate ID -> field
	employer, err := users.Create(ctx, conn, "seed.employer@example.com", users.RoleEmployer)
	if err != nil {
		return err
	}
	users.SetFirstName(ctx, conn, employer.ID, "Sam")

	start := time.Now()
	var people []profiles.Profile
	for i, s := range candidates {
		u, err := users.Create(ctx, conn, fmt.Sprintf("seed.candidate.%d@example.com", i+1), users.RoleCandidate)
		if err != nil {
			return err
		}
		now := time.Now()
		p := profiles.Profile{
			UserID: u.ID, ParseStatus: profiles.StatusDone, OnboardedAt: &now,
			CV: profiles.CV{FullName: s.name, Headline: s.headline, CurrentTitle: s.headline, Seniority: s.seniority,
				YearsExperience: s.years, Skills: s.skills, Summary: s.summary,
				Experience: []profiles.Experience{}, Education: []profiles.Education{}},
			RoleTypes: []string{}, Locations: []string{}, HiddenFrom: []string{},
			WorkMode: "any", PayCurrency: "USD", EmployerDisplay: "name", ShowPay: true,
		}
		if err := conn.Create(&p).Error; err != nil {
			return err
		}
		if err := matching.EmbedOwner(ctx, conn, embedder, matching.OwnerCandidate, p.UserID); err != nil {
			return err
		}
		field[p.UserID] = s.field
		people = append(people, p)
	}
	var live []hiring.Card
	for _, s := range cards {
		c := hiring.Card{EmployerUserID: employer.ID, Status: hiring.StatusLive, Title: s.title, CompanyName: s.company,
			Seniority: s.seniority, WorkMode: s.mode, Skills: s.skills, Summary: s.summary, PayCurrency: "USD"}
		if err := conn.Create(&c).Error; err != nil {
			return err
		}
		if err := matching.EmbedOwner(ctx, conn, embedder, matching.OwnerHiringCard, c.ID); err != nil {
			return err
		}
		field[c.ID] = s.field
		live = append(live, c)
	}
	fmt.Printf("seeded %d candidates and %d hiring cards, embedded in %.1f s\n\n",
		len(people), len(live), time.Since(start).Seconds())

	return report(ctx, conn, people, live, field)
}

// report prints each timeline's top results and how many fit the field.
func report(ctx context.Context, conn *gorm.DB, people []profiles.Profile, live []hiring.Card, field map[string]string) error {
	var p1, ndcg float64
	var slowest time.Duration
	fmt.Println("Candidate timelines (top 3):")
	for _, p := range people {
		t0 := time.Now()
		matches, err := matching.CandidateTimeline(ctx, conn, p)
		slowest = max(slowest, time.Since(t0))
		if err != nil {
			return err
		}
		var labels []int
		var top []string
		for i, m := range matches {
			labels = append(labels, b2i(field[m.Card.ID] == field[p.UserID]))
			if i < 3 {
				top = append(top, fmt.Sprintf("%s (%.2f)", m.Card.Title, m.Score))
			}
		}
		p1 += float64(first(labels))
		ndcg += nDCG(labels, 5)
		fmt.Printf("  %-26s -> %v\n", p.CV.Headline, top)
	}
	fmt.Printf("  precision@1 %.2f, nDCG@5 %.2f\n\n", p1/float64(len(people)), ndcg/float64(len(people)))

	p1, ndcg = 0, 0
	fmt.Println("Hiring card timelines (top 3):")
	for _, c := range live {
		t0 := time.Now()
		matches, err := matching.HiringCardTimeline(ctx, conn, c)
		slowest = max(slowest, time.Since(t0))
		if err != nil {
			return err
		}
		var labels []int
		var top []string
		for i, m := range matches {
			labels = append(labels, b2i(field[m.Profile.UserID] == field[c.ID]))
			if i < 3 {
				top = append(top, fmt.Sprintf("%s (%.2f)", m.Profile.CV.Headline, m.Score))
			}
		}
		p1 += float64(first(labels))
		ndcg += nDCG(labels, 5)
		fmt.Printf("  %-26s -> %v\n", c.Title, top)
	}
	fmt.Printf("  precision@1 %.2f, nDCG@5 %.2f\n\n", p1/float64(len(live)), ndcg/float64(len(live)))
	fmt.Printf("slowest timeline query: %s\n\n", slowest.Round(time.Millisecond))

	return cosineSpread(ctx, conn, field)
}

// cosineSpread prints the cosines of same-field and cross-field pairs, to
// set the floor and ceiling of s_vector for this model.
func cosineSpread(ctx context.Context, conn *gorm.DB, field map[string]string) error {
	var pairs []struct {
		Candidate string
		Card      string
		Cosine    float64
	}
	err := conn.WithContext(ctx).Raw(`
		SELECT c.owner_id::text AS candidate, j.owner_id::text AS card, 1 - (c.embedding <=> j.embedding) AS cosine
		FROM embeddings c JOIN embeddings j ON j.owner_type = 'hiring_card' AND j.model = c.model
		WHERE c.owner_type = 'candidate' AND c.model = ?`, embed.Model).Scan(&pairs).Error
	if err != nil {
		return err
	}
	var same, other []float64
	for _, p := range pairs {
		f, g := field[p.Candidate], field[p.Card]
		if f == "" || g == "" {
			continue // not seeded by this run
		}
		if f == g {
			same = append(same, p.Cosine)
		} else {
			other = append(other, p.Cosine)
		}
	}
	fmt.Println("Cosine spread for this model:")
	fmt.Printf("  same field:      %s\n", spread(same))
	fmt.Printf("  different field: %s\n", spread(other))
	return nil
}

func spread(xs []float64) string {
	if len(xs) == 0 {
		return "none"
	}
	sort.Float64s(xs)
	var sum float64
	for _, x := range xs {
		sum += x
	}
	pct := func(p float64) float64 { return xs[int(p*float64(len(xs)-1))] }
	return fmt.Sprintf("n=%d min %.3f p10 %.3f median %.3f mean %.3f p90 %.3f max %.3f",
		len(xs), xs[0], pct(0.1), pct(0.5), sum/float64(len(xs)), pct(0.9), xs[len(xs)-1])
}

// nDCG at k for 0/1 labels (paper, section 8).
func nDCG(labels []int, k int) float64 {
	dcg, ideal := 0.0, 0.0
	relevant := 0
	for i, r := range labels {
		if i < k {
			dcg += float64(r) / math.Log2(float64(i+2))
		}
		relevant += r
	}
	for i := 0; i < min(k, relevant); i++ {
		ideal += 1 / math.Log2(float64(i+2))
	}
	if ideal == 0 {
		return 0
	}
	return dcg / ideal
}

func removeOldSeed(ctx context.Context, conn *gorm.DB) error {
	return conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ids := tx.Table("users").Select("id").Where("email LIKE 'seed.%@example.com'")
		cardIDs := tx.Table("hiring_cards").Select("id").Where("employer_user_id IN (?)", ids)
		if err := tx.Exec(`DELETE FROM embeddings WHERE owner_id IN (?) OR owner_id IN (?)`, ids, cardIDs).Error; err != nil {
			return err
		}
		return tx.Exec(`DELETE FROM users WHERE email LIKE 'seed.%@example.com'`).Error
	})
}

func first(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	return xs[0]
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
