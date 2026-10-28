// Package matching ranks hiring cards for candidates and candidates for
// hiring cards. The formulas are explained, with worked numbers, in the
// embeddings paper (docs/EMBEDDINGS.md, section 6).
package matching

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Spec weights for the fit score.
const (
	weightVector    = 0.45
	weightSkills    = 0.25
	weightSeniority = 0.15
	weightPay       = 0.15
)

// Cosine floor and ceiling for rescaling s_vector; to be tuned once we see
// real cosines from the model.
const (
	cosineFloor   = 0.2
	cosineCeiling = 0.9
)

var levels = []string{"intern", "junior", "mid", "senior", "lead", "principal"}

// Side is what the scoring needs from either card.
type Side struct {
	Skills    []string
	Seniority string
}

// Pay is a candidate's minimum and a card's range (0 means not given).
type Pay struct {
	CandidateMin int
	CardMin      int
	CardMax      int
}

// Scores are the sub-scores of one pair, each in [0, 1].
type Scores struct {
	Vector    float64
	Skills    float64
	Seniority float64
	Pay       float64
	Fit       float64
	// Shared lists the skills both sides have, for "why this card".
	Shared []string
}

// Score computes the fit of a candidate and a hiring card from their
// cosine similarity and their fields (paper, section 6.3).
func Score(candidate, card Side, pay Pay, cosine float64) Scores {
	s := Scores{
		Vector:    clamp((cosine - cosineFloor) / (cosineCeiling - cosineFloor)),
		Seniority: seniorityScore(candidate.Seniority, card.Seniority),
		Pay:       payScore(pay),
	}
	s.Skills, s.Shared = skillsScore(candidate.Skills, card.Skills)
	s.Fit = weightVector*s.Vector + weightSkills*s.Skills + weightSeniority*s.Seniority + weightPay*s.Pay
	return s
}

// PairScore is the harmonic mean of both sides' chance of liking each other
// (paper, section 6.4). At cold start both chances are the fit.
func PairScore(pCandidate, pEmployer float64) float64 {
	if pCandidate+pEmployer == 0 {
		return 0
	}
	return 2 * pCandidate * pEmployer / (pCandidate + pEmployer)
}

// skillsScore is the share of the card's skills the candidate has.
func skillsScore(candidate, card []string) (float64, []string) {
	if len(card) == 0 {
		return 0.5, nil
	}
	have := map[string]bool{}
	for _, skill := range candidate {
		have[strings.ToLower(strings.TrimSpace(skill))] = true
	}
	var shared []string
	for _, skill := range card {
		if have[strings.ToLower(strings.TrimSpace(skill))] {
			shared = append(shared, skill)
		}
	}
	return float64(len(shared)) / float64(len(card)), shared
}

// seniorityScore is 1 for the same level, 0.5 one level apart, 0 beyond;
// 0.5 when either side is unknown.
func seniorityScore(a, b string) float64 {
	i, j := slices.Index(levels, a), slices.Index(levels, b)
	if i < 0 || j < 0 {
		return 0.5
	}
	return max(0, 1-float64(abs(i-j))/2)
}

// payScore is the share of the card's range at or above the candidate's
// minimum (pay that is too low is removed earlier by the filters).
func payScore(p Pay) float64 {
	switch {
	case p.CandidateMin == 0 || p.CardMax == 0:
		return 0.5
	case p.CandidateMin <= p.CardMin:
		return 1
	case p.CandidateMin > p.CardMax:
		return 0
	}
	return float64(p.CardMax-p.CandidateMin) / float64(p.CardMax-p.CardMin)
}

// Reasons turns the sub-scores into short words for "why this card", at
// most three. Shared skills come first (the most telling reason), then the
// strongest of the rest.
func Reasons(s Scores) []string {
	type reason struct {
		score float64
		text  string
	}
	var all []reason
	if len(s.Shared) > 0 {
		all = append(all, reason{2, "Skills match: " + strings.Join(first(s.Shared, 3), ", ")})
	}
	if s.Vector >= 0.5 {
		all = append(all, reason{s.Vector, "Similar kind of work"})
	}
	if s.Seniority == 1 {
		all = append(all, reason{s.Seniority, "Same seniority"})
	}
	if s.Pay == 1 {
		all = append(all, reason{s.Pay, "Pay fits"})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].score > all[j].score })
	out := []string{}
	for _, r := range first(all, 3) {
		out = append(out, r.text)
	}
	return out
}

func clamp(x float64) float64 { return min(1, max(0, x)) }

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func first[T any](items []T, n int) []T {
	if len(items) > n {
		return items[:n]
	}
	return items
}

// String is a short summary for logs and tests.
func (s Scores) String() string {
	return fmt.Sprintf("fit %.3f (vector %.3f, skills %.3f, seniority %.3f, pay %.3f)", s.Fit, s.Vector, s.Skills, s.Seniority, s.Pay)
}
