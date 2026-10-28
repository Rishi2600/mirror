package matching

import (
	"context"
	"errors"
	"sort"

	"gorm.io/gorm"

	"mirror/server/internal/embed"
	"mirror/server/internal/hiring"
	"mirror/server/internal/profiles"
)

// nearest is how many rows the vector search returns before scoring
// (paper, section 6.2).
const nearest = 200

// maxInARow caps cards from one employer in a row on a candidate's
// timeline (paper, section 6.5).
const maxInARow = 3

// ErrNoVector means the card's vector is not made yet (it takes a second or
// two after the card is saved).
var ErrNoVector = errors.New("this card is still being prepared for matching; try again in a moment")

// Hard filters shared by both timelines (paper, section 6.1). They compare
// a candidate profile "p" with a hiring card "h".
const hardFilters = `
	h.status = 'live'
	AND p.onboarded_at IS NOT NULL
	AND h.employer_user_id <> p.user_id
	AND (h.pay_max = 0 OR p.min_pay = 0 OR h.pay_max >= p.min_pay)
	AND (p.work_mode = 'any' OR h.work_mode = p.work_mode
		OR (p.work_mode = 'hybrid' AND h.work_mode = 'remote')
		OR (p.work_mode = 'onsite' AND h.work_mode = 'hybrid'))
	AND (NOT p.needs_visa OR h.visa_sponsorship)
	AND (h.seniority = '' OR coalesce(p.cv->>'seniority', '') = ''
		OR abs(array_position(ARRAY['intern','junior','mid','senior','lead','principal'], h.seniority)
			- array_position(ARRAY['intern','junior','mid','senior','lead','principal'], p.cv->>'seniority')) <= 1)
	AND NOT EXISTS (
		SELECT 1 FROM jsonb_array_elements_text(p.hidden_from) AS hidden(name)
		WHERE h.company_name <> '' AND lower(trim(hidden.name)) = lower(trim(h.company_name)))`

// JobMatch is one hiring card on a candidate's timeline.
type JobMatch struct {
	Card      hiring.Card
	FirstName string
	Scores    Scores
	// Score is the pair score; at cold start it equals the fit.
	Score float64
}

// CandidateMatch is one candidate on a hiring card's timeline.
type CandidateMatch struct {
	Profile profiles.Profile
	Scores  Scores
	Score   float64
}

// CandidateTimeline returns the live hiring cards that fit a candidate,
// best first.
func CandidateTimeline(ctx context.Context, db *gorm.DB, p profiles.Profile) ([]JobMatch, error) {
	if !hasVector(ctx, db, OwnerCandidate, p.UserID) {
		return nil, ErrNoVector
	}
	var rows []struct {
		hiring.Card `gorm:"embedded"`
		FirstName   string
		Cosine      float64
	}
	err := db.WithContext(ctx).Raw(`
		SELECT h.*, u.first_name, 1 - (he.embedding <=> pe.embedding) AS cosine
		FROM candidate_profiles p
		JOIN embeddings pe ON pe.owner_type = 'candidate' AND pe.owner_id = p.user_id AND pe.model = @model
		JOIN hiring_cards h ON true
		JOIN embeddings he ON he.owner_type = 'hiring_card' AND he.owner_id = h.id AND he.model = @model
		JOIN users u ON u.id = h.employer_user_id
		WHERE p.user_id = @user AND `+hardFilters+`
		ORDER BY he.embedding <=> pe.embedding
		LIMIT @nearest`,
		map[string]any{"model": embed.Model, "user": p.UserID, "nearest": nearest}).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	matches := make([]JobMatch, 0, len(rows))
	for _, r := range rows {
		s := scorePair(p, r.Card, r.Cosine)
		matches = append(matches, JobMatch{Card: r.Card, FirstName: r.FirstName, Scores: s, Score: PairScore(s.Fit, s.Fit)})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
	return spreadEmployers(matches), nil
}

// HiringCardTimeline returns the finished candidate cards that fit a live
// hiring card, best first. (Candidates who already liked the card will go
// first once swipes exist, in chunk 9.)
func HiringCardTimeline(ctx context.Context, db *gorm.DB, c hiring.Card) ([]CandidateMatch, error) {
	if !hasVector(ctx, db, OwnerHiringCard, c.ID) {
		return nil, ErrNoVector
	}
	var rows []struct {
		profiles.Profile `gorm:"embedded"`
		Cosine           float64
	}
	err := db.WithContext(ctx).Raw(`
		SELECT p.*, 1 - (pe.embedding <=> he.embedding) AS cosine
		FROM hiring_cards h
		JOIN embeddings he ON he.owner_type = 'hiring_card' AND he.owner_id = h.id AND he.model = @model
		JOIN candidate_profiles p ON true
		JOIN embeddings pe ON pe.owner_type = 'candidate' AND pe.owner_id = p.user_id AND pe.model = @model
		WHERE h.id = @card AND `+hardFilters+`
		ORDER BY pe.embedding <=> he.embedding
		LIMIT @nearest`,
		map[string]any{"model": embed.Model, "card": c.ID, "nearest": nearest}).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	matches := make([]CandidateMatch, 0, len(rows))
	for _, r := range rows {
		s := scorePair(r.Profile, c, r.Cosine)
		matches = append(matches, CandidateMatch{Profile: r.Profile, Scores: s, Score: PairScore(s.Fit, s.Fit)})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
	return matches, nil
}

func scorePair(p profiles.Profile, c hiring.Card, cosine float64) Scores {
	return Score(
		Side{Skills: p.CV.Skills, Seniority: p.CV.Seniority},
		Side{Skills: c.Skills, Seniority: c.Seniority},
		Pay{CandidateMin: p.MinPay, CardMin: c.PayMin, CardMax: c.PayMax},
		cosine,
	)
}

func hasVector(ctx context.Context, db *gorm.DB, ownerType, ownerID string) bool {
	var n int64
	db.WithContext(ctx).Raw(`SELECT count(*) FROM embeddings WHERE owner_type = ? AND owner_id = ? AND model = ?`,
		ownerType, ownerID, embed.Model).Scan(&n)
	return n > 0
}

// spreadEmployers keeps the order but moves a card down when the cards
// before it are already maxInARow cards from the same employer.
func spreadEmployers(matches []JobMatch) []JobMatch {
	out := make([]JobMatch, 0, len(matches))
	waiting := matches
	for len(waiting) > 0 {
		picked := 0
		for i, m := range waiting {
			if !tooMany(out, m.Card.EmployerUserID) {
				picked = i
				break
			}
		}
		out = append(out, waiting[picked])
		waiting = append(waiting[:picked:picked], waiting[picked+1:]...)
	}
	return out
}

func tooMany(out []JobMatch, employerID string) bool {
	if len(out) < maxInARow {
		return false
	}
	for _, m := range out[len(out)-maxInARow:] {
		if m.Card.EmployerUserID != employerID {
			return false
		}
	}
	return true
}
