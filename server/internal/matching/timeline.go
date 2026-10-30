package matching

import (
	"context"
	"errors"
	"sort"
	"time"

	"gorm.io/gorm"

	"mirror/server/internal/embed"
	"mirror/server/internal/hiring"
	"mirror/server/internal/profiles"
	"mirror/server/internal/selectivity"
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
// a candidate profile "p" with a hiring card "h". p.seniority and
// p.hidden_keys are generated columns (migration 00009), so no row's cv JSON
// is unpacked.
const hardFilters = `
	h.status = 'live'
	AND p.onboarded_at IS NOT NULL
	AND h.employer_user_id <> p.user_id
	AND (h.pay_max = 0 OR p.min_pay = 0 OR h.pay_max >= p.min_pay)
	AND (p.work_mode = 'any' OR h.work_mode = p.work_mode
		OR (p.work_mode = 'hybrid' AND h.work_mode = 'remote')
		OR (p.work_mode = 'onsite' AND h.work_mode = 'hybrid'))
	AND (NOT p.needs_visa OR h.visa_sponsorship)
	AND (h.seniority = '' OR p.seniority = ''
		OR abs(array_position(ARRAY['intern','junior','mid','senior','lead','principal'], h.seniority)
			- array_position(ARRAY['intern','junior','mid','senior','lead','principal'], p.seniority)) <= 1)
	AND NOT (h.company_name <> '' AND jsonb_exists(p.hidden_keys, lower(trim(h.company_name))))`

// notSwipedBy leaves out pairs the viewing side already swiped, and pairs
// either side skipped (a skip ends the pair for both).
func notSwipedBy(side string) string {
	return `
	AND NOT EXISTS (
		SELECT 1 FROM swipes s
		WHERE s.hiring_card_id = h.id AND s.candidate_user_id = p.user_id
			AND (s.side = '` + side + `' OR s.direction = 'left'))`
}

// JobMatch is one hiring card on a candidate's timeline.
type JobMatch struct {
	Card      hiring.Card
	FirstName string
	Scores    Scores
	// Score is the pair score; at cold start it equals the fit.
	Score float64
	// Rank orders the timeline: the score, lower for an employer who ignores
	// their matches. Score stays the true fit shown on the card.
	Rank float64
}

// CandidateMatch is one candidate on a hiring card's timeline.
type CandidateMatch struct {
	Profile profiles.Profile
	Scores  Scores
	Score   float64
	// Rank orders the timeline: the score times the candidate's Selectivity
	// weight. Score stays the true fit shown on the card.
	Rank float64
	// Liked is true when the candidate already liked this card; they come
	// first, because one more like makes a match. A low score takes this
	// away until it recovers.
	Liked bool
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
		WHERE p.user_id = @user AND `+hardFilters+notSwipedBy("candidate")+`
		ORDER BY he.embedding <=> pe.embedding
		LIMIT @nearest`,
		map[string]any{"model": embed.Model, "user": p.UserID, "nearest": nearest}).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	employers := make([]string, len(rows))
	for i, r := range rows {
		employers[i] = r.EmployerUserID
	}
	ghosting, err := selectivity.Ghosting(ctx, db, employers, time.Now())
	if err != nil {
		return nil, err
	}

	matches := make([]JobMatch, 0, len(rows))
	for _, r := range rows {
		s := scorePair(p, r.Card, r.Cosine)
		m := JobMatch{Card: r.Card, FirstName: r.FirstName, Scores: s, Score: PairScore(s.Fit, s.Fit)}
		m.Rank = m.Score
		if ghosting[r.EmployerUserID] {
			m.Rank *= selectivity.GhostWeight
		}
		matches = append(matches, m)
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Rank > matches[j].Rank })
	return spreadEmployers(matches), nil
}

// HiringCardTimeline returns the finished candidate cards that fit a live
// hiring card: those who already liked the card first, then the rest, each
// group best first.
func HiringCardTimeline(ctx context.Context, db *gorm.DB, c hiring.Card) ([]CandidateMatch, error) {
	if !hasVector(ctx, db, OwnerHiringCard, c.ID) {
		return nil, ErrNoVector
	}
	var rows []struct {
		profiles.Profile `gorm:"embedded"`
		Cosine           float64
		Liked            bool
		Selectivity      float64
	}
	err := db.WithContext(ctx).Raw(`
		SELECT p.*, 1 - (pe.embedding <=> he.embedding) AS cosine,
			EXISTS (SELECT 1 FROM swipes s WHERE s.hiring_card_id = h.id AND s.candidate_user_id = p.user_id
				AND s.side = 'candidate' AND s.direction = 'right')
				AND NOT COALESCE(sel.low_since <= @low_before, false) AS liked,
			COALESCE(sel.score, @new_score) AS selectivity
		FROM hiring_cards h
		JOIN embeddings he ON he.owner_type = 'hiring_card' AND he.owner_id = h.id AND he.model = @model
		JOIN candidate_profiles p ON true
		JOIN embeddings pe ON pe.owner_type = 'candidate' AND pe.owner_id = p.user_id AND pe.model = @model
		LEFT JOIN selectivity_scores sel ON sel.user_id = p.user_id
		WHERE h.id = @card AND `+hardFilters+notSwipedBy("employer")+`
		ORDER BY liked DESC, pe.embedding <=> he.embedding
		LIMIT @nearest`,
		map[string]any{
			"model": embed.Model, "card": c.ID, "nearest": nearest,
			"low_before": time.Now().Add(-selectivity.LowAfter), "new_score": selectivity.NewScore,
		}).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	matches := make([]CandidateMatch, 0, len(rows))
	for _, r := range rows {
		s := scorePair(r.Profile, c, r.Cosine)
		score := PairScore(s.Fit, s.Fit)
		rank := score * selectivity.Weight(r.Selectivity)
		matches = append(matches, CandidateMatch{Profile: r.Profile, Scores: s, Score: score, Rank: rank, Liked: r.Liked})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Liked != matches[j].Liked {
			return matches[i].Liked
		}
		return matches[i].Rank > matches[j].Rank
	})
	return matches, nil
}

// Fit returns the fit of a candidate and a hiring card, or nil until both
// have their vectors.
func Fit(ctx context.Context, db *gorm.DB, p profiles.Profile, c hiring.Card) (*float64, error) {
	var cosines []float64
	err := db.WithContext(ctx).Raw(`
		SELECT 1 - (pe.embedding <=> he.embedding)
		FROM embeddings pe, embeddings he
		WHERE pe.owner_type = 'candidate' AND pe.owner_id = @user AND pe.model = @model
			AND he.owner_type = 'hiring_card' AND he.owner_id = @card AND he.model = @model`,
		map[string]any{"user": p.UserID, "card": c.ID, "model": embed.Model}).Scan(&cosines).Error
	if err != nil || len(cosines) == 0 {
		return nil, err
	}
	fit := scorePair(p, c, cosines[0]).Fit
	return &fit, nil
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
