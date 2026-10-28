package matching

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 0.0005 }

var jordan = Side{
	Skills:    []string{"Go", "Python", "PostgreSQL", "Redis", "Kubernetes", "Docker", "gRPC", "AWS"},
	Seniority: "senior",
}

// Worked example 1 of the embeddings paper (section 6.3).
func TestPaperExampleGoPaymentsCard(t *testing.T) {
	card := Side{Skills: []string{"Go", "SQL", "PostgreSQL", "Kubernetes", "payment APIs"}, Seniority: "senior"}
	s := Score(jordan, card, Pay{CandidateMin: 90000, CardMin: 90000, CardMax: 120000}, 0.82)

	if !near(s.Vector, 0.8857) || !near(s.Skills, 0.6) || s.Seniority != 1 || s.Pay != 1 || !near(s.Fit, 0.849) {
		t.Errorf("got %s, want fit 0.849 (vector 0.886, skills 0.6, seniority 1, pay 1)", s)
	}
}

// Worked example 2 of the embeddings paper.
func TestPaperExamplePythonCard(t *testing.T) {
	card := Side{Skills: []string{"Python", "Django", "PostgreSQL", "AWS"}, Seniority: "mid"}
	s := Score(jordan, card, Pay{CandidateMin: 90000, CardMin: 70000, CardMax: 100000}, 0.70)

	if !near(s.Vector, 0.7143) || !near(s.Skills, 0.75) || s.Seniority != 0.5 || !near(s.Pay, 0.3333) || !near(s.Fit, 0.634) {
		t.Errorf("got %s, want fit 0.634", s)
	}
}

// The paper's table in section 6.4: the harmonic mean prefers the
// balanced pair.
func TestPairScore(t *testing.T) {
	if got := PairScore(0.9, 0.3); !near(got, 0.45) {
		t.Errorf("one-sided pair = %.3f, want 0.45", got)
	}
	if got := PairScore(0.6, 0.6); !near(got, 0.6) {
		t.Errorf("balanced pair = %.3f, want 0.60", got)
	}
	if PairScore(0, 0) != 0 {
		t.Error("PairScore(0, 0) is not 0")
	}
}

func TestEdgeCases(t *testing.T) {
	s := Score(Side{}, Side{}, Pay{}, 0.1)
	if s.Vector != 0 || s.Skills != 0.5 || s.Seniority != 0.5 || s.Pay != 0.5 {
		t.Errorf("unknowns should be neutral and low cosine 0: %s", s)
	}
	if s := Score(jordan, jordan, Pay{}, 0.99); s.Vector != 1 {
		t.Errorf("cosine above the ceiling should give 1, got %.3f", s.Vector)
	}
	if got := seniorityScore("intern", "lead"); got != 0 {
		t.Errorf("far-apart seniority = %.2f, want 0", got)
	}
}

func TestReasons(t *testing.T) {
	card := Side{Skills: []string{"Go", "PostgreSQL", "Kubernetes", "Terraform"}, Seniority: "senior"}
	s := Score(jordan, card, Pay{CandidateMin: 90000, CardMin: 100000, CardMax: 120000}, 0.85)
	got := Reasons(s)
	if len(got) != 3 || got[0] != "Skills match: Go, PostgreSQL, Kubernetes" {
		t.Errorf("reasons = %q, want shared skills first, then two more", got)
	}
	if got := Reasons(Score(Side{}, Side{Skills: []string{"Rust"}}, Pay{}, 0.1)); len(got) != 0 {
		t.Errorf("a weak pair has reasons %q, want none", got)
	}
}
