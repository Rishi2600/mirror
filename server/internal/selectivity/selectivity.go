// Package selectivity keeps a like meaningful. Each candidate has a score
// from how they swipe; it weights them on employer timelines, sets their
// daily like budget, and can pause their likes. Employers who ignore their
// matches rank lower for candidates. The reasoning and worked numbers are in
// the paper (docs/SELECTIVITY.md).
package selectivity

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"mirror/server/internal/events"
	"mirror/server/internal/users"
)

// The spec's starting defaults, to tune on real swipe data.
const (
	targetRightRate = 0.30
	dwellRefMS      = 4000
	window          = 7 * 24 * time.Hour

	// A new candidate holds NewScore until they have newSwipes swipes.
	NewScore  = 0.7
	newSwipes = 50
	// With fewer swipes in the window there is too little to judge, so the
	// last score stands.
	minWindowSwipes = 10

	// What the formula uses until there is data.
	noReplyData   = 0.5
	noFitData     = 0.5
	noDwellDataMS = dwellRefMS / 2

	budget     = 15
	highBudget = 20
	highScore  = 0.8

	burstLikes   = 8
	burstWindow  = 60 * time.Second
	burstDwellMS = 1500
	burstPause   = 10 * time.Minute

	lowScore = 0.4
	LowAfter = 3 * 24 * time.Hour

	// A reply is a message within this many days of the match.
	replyWithin = 3

	ghostWindow    = 14 * 24 * time.Hour
	ghostReplyRate = 0.30
	// One ignored match says little, so a few are needed.
	ghostMinMatches = 3
	// GhostWeight scales a ghosting employer's cards on candidate timelines.
	GhostWeight = 0.8
)

// Inputs are what the score is computed from, over the last 7 days.
type Inputs struct {
	RightRate     float64 // likes divided by all swipes
	MeanFit       float64 // mean fit of the liked cards
	MedianDwellMS float64 // median time on a card
	ReplyRate     float64 // share of matches the candidate wrote back on
}

// Score is the spec's formula; the result is between 0 and 1.
func Score(in Inputs) float64 {
	over := max(0, in.RightRate-targetRightRate) / (1 - targetRightRate)
	dwell := min(in.MedianDwellMS/dwellRefMS, 1)
	return 0.35*(1-over) + 0.25*in.MeanFit + 0.20*dwell + 0.20*in.ReplyRate
}

// Weight is what a candidate's rank is multiplied by on employer timelines.
// It never goes below 0.5, so fit always counts.
func Weight(score float64) float64 {
	return 0.5 + 0.5*score
}

// DailyBudget is how many likes a candidate gets a day.
func DailyBudget(score float64) int {
	if score >= highScore {
		return highBudget
	}
	return budget
}

// State is a candidate's row in selectivity_scores.
type State struct {
	UserID        string `gorm:"primaryKey"`
	Score         float64
	RightRate     float64
	MeanFit       float64
	MedianDwellMS float64 `gorm:"column:median_dwell_ms"`
	ReplyRate     float64
	Swipes7d      int `gorm:"column:swipes_7d"`
	LowSince      *time.Time
	PausedUntil   *time.Time
	UpdatedAt     time.Time
}

func (State) TableName() string { return "selectivity_scores" }

func load(ctx context.Context, db *gorm.DB, userID string) (State, error) {
	var s State
	err := db.WithContext(ctx).Where("user_id = ?", userID).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return State{UserID: userID, Score: NewScore}, nil
	}
	return s, err
}

// Limited means a like was refused until a certain time.
type Limited struct {
	Reason string
	Until  time.Time
}

func (e *Limited) Error() string { return e.Reason }

// CheckLike refuses a like while likes are paused or the day's budget is
// used. Call it inside the swipe's transaction, holding the candidate lock.
func CheckLike(ctx context.Context, tx *gorm.DB, user users.User, now time.Time) error {
	likes, err := LikesFor(ctx, tx, user, now)
	if err != nil {
		return err
	}
	if likes.PausedUntil != nil {
		return &Limited{Reason: "likes are paused for now; you can still skip", Until: *likes.PausedUntil}
	}
	if likes.Left == 0 {
		return &Limited{Reason: "you have used today's likes; you can still skip", Until: likes.ResetsAt}
	}
	return nil
}

// Likes is where a candidate stands today. The browser turns it into nudges.
type Likes struct {
	DailyBudget int        `json:"daily_budget"`
	Left        int        `json:"left"`
	ResetsAt    time.Time  `json:"resets_at"`
	PausedUntil *time.Time `json:"paused_until"`
	LowScore    bool       `json:"low_score"`
}

// LikesFor returns the candidate's likes today.
func LikesFor(ctx context.Context, db *gorm.DB, user users.User, now time.Time) (Likes, error) {
	s, err := load(ctx, db, user.ID)
	if err != nil {
		return Likes{}, err
	}
	dayStart, nextDay := localDay(user.TimeZone, now)
	var used int64
	err = db.WithContext(ctx).Table("swipes").
		Where("actor_user_id = ? AND side = 'candidate' AND direction = 'right' AND created_at >= ?", user.ID, dayStart).
		Count(&used).Error
	if err != nil {
		return Likes{}, err
	}
	likes := Likes{
		DailyBudget: DailyBudget(s.Score),
		ResetsAt:    nextDay,
		LowScore:    s.LowSince != nil && now.Sub(*s.LowSince) >= LowAfter,
	}
	likes.Left = max(0, likes.DailyBudget-int(used))
	if s.PausedUntil != nil && s.PausedUntil.After(now) {
		likes.PausedUntil = s.PausedUntil
	}
	return likes, nil
}

// localDay returns the start of today and of tomorrow in the time zone. An
// empty or unknown zone means UTC.
func localDay(zone string, now time.Time) (start, next time.Time) {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		loc = time.UTC
	}
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc), time.Date(y, m, d+1, 0, 0, 0, 0, loc)
}

// AfterSwipe runs inside a candidate's swipe transaction, holding the
// candidate lock: it pauses a burst of unread likes, and brings the score up
// to date.
func AfterSwipe(ctx context.Context, tx *gorm.DB, userID string, now time.Time) error {
	s, err := load(ctx, tx, userID)
	if err != nil {
		return err
	}
	burst, err := isBurst(ctx, tx, userID)
	if err != nil {
		return err
	}
	if burst {
		until := now.Add(burstPause)
		s.PausedUntil = &until
	}
	previous := s.Score
	first := s.UpdatedAt.IsZero()
	if err := recompute(ctx, tx, &s, now); err != nil {
		return err
	}
	s.UpdatedAt = now
	err = tx.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&s).Error
	if err != nil {
		return err
	}
	// The history only needs the changes, not one row per swipe.
	if !first && math.Abs(s.Score-previous) < 0.01 {
		return nil
	}
	return events.LogSelectivity(ctx, tx, events.SelectivitySnapshot{
		UserID: s.UserID, Score: s.Score, RightRate: s.RightRate, MeanFit: s.MeanFit,
		MedianDwellMS: s.MedianDwellMS, ReplyRate: s.ReplyRate, Swipes7d: s.Swipes7d,
	})
}

// isBurst reports whether the candidate's last likes came too fast and with
// too little reading: 8 in 60 seconds, with a median under 1.5 seconds.
func isBurst(ctx context.Context, tx *gorm.DB, userID string) (bool, error) {
	var row struct {
		Likes    int
		SpanMS   float64
		MedianMS *float64
	}
	err := tx.WithContext(ctx).Raw(`
		SELECT count(*) AS likes,
			COALESCE(extract(epoch FROM max(created_at) - min(created_at)) * 1000, 0) AS span_ms,
			percentile_cont(0.5) WITHIN GROUP (ORDER BY dwell_ms) AS median_ms
		FROM (SELECT created_at, dwell_ms FROM swipes
			WHERE actor_user_id = ? AND side = 'candidate' AND direction = 'right'
			ORDER BY created_at DESC LIMIT ?) AS recent`,
		userID, burstLikes).Scan(&row).Error
	if err != nil || row.Likes < burstLikes || row.MedianMS == nil {
		return false, err
	}
	return row.SpanMS <= float64(burstWindow.Milliseconds()) && *row.MedianMS < burstDwellMS, nil
}

// recompute sets the inputs and score from the last 7 days of swipes.
func recompute(ctx context.Context, tx *gorm.DB, s *State, now time.Time) error {
	var row struct {
		Swipes   int
		Likes    int
		MeanFit  *float64
		MedianMS *float64
	}
	err := tx.WithContext(ctx).Raw(`
		SELECT count(*) AS swipes,
			count(*) FILTER (WHERE direction = 'right') AS likes,
			avg(fit) FILTER (WHERE direction = 'right') AS mean_fit,
			percentile_cont(0.5) WITHIN GROUP (ORDER BY dwell_ms) AS median_ms
		FROM swipes
		WHERE actor_user_id = ? AND side = 'candidate' AND created_at >= ?`,
		s.UserID, now.Add(-window)).Scan(&row).Error
	if err != nil {
		return err
	}
	var everSwiped int64
	err = tx.WithContext(ctx).Raw(`SELECT count(*) FROM (
		SELECT 1 FROM swipes WHERE actor_user_id = ? AND side = 'candidate' LIMIT ?) AS seen`,
		s.UserID, newSwipes).Scan(&everSwiped).Error
	if err != nil {
		return err
	}
	reply, err := replyRate(ctx, tx, s.UserID, now)
	if err != nil {
		return err
	}

	s.Swipes7d = row.Swipes
	s.RightRate = 0
	if row.Swipes > 0 {
		s.RightRate = float64(row.Likes) / float64(row.Swipes)
	}
	s.MeanFit, s.MedianDwellMS, s.ReplyRate = noFitData, noDwellDataMS, reply
	if row.MeanFit != nil {
		s.MeanFit = *row.MeanFit
	}
	if row.MedianMS != nil {
		s.MedianDwellMS = *row.MedianMS
	}

	switch {
	case everSwiped < newSwipes:
		s.Score = NewScore
	case row.Swipes >= minWindowSwipes:
		s.Score = Score(Inputs{s.RightRate, s.MeanFit, s.MedianDwellMS, s.ReplyRate})
	}
	switch {
	case s.Score >= lowScore:
		s.LowSince = nil
	case s.LowSince == nil:
		s.LowSince = &now
	}
	return nil
}

// replyCounts counts matches that are old enough to judge, and how many of
// them the person wrote on within 3 days. A match ended before then is left
// out: there was no fair chance to reply.
const replyCounts = `
	SELECT %[1]s AS user_id, count(*) AS total,
		count(*) FILTER (WHERE EXISTS (
			SELECT 1 FROM messages x
			WHERE x.match_id = m.id AND x.sender_user_id = m.%[1]s
				AND x.created_at <= m.matched_at + make_interval(days => @days))) AS replied
	FROM matches m
	WHERE m.%[1]s IN @users
		AND m.matched_at >= @since AND m.matched_at <= @until
		AND (m.unmatched_at IS NULL OR m.unmatched_at >= m.matched_at + make_interval(days => @days))
	GROUP BY %[1]s`

type replyRow struct {
	UserID  string
	Total   int
	Replied int
}

// replies counts for matches whose 3 days to reply ended within the period.
func replies(ctx context.Context, db *gorm.DB, column string, userIDs []string, period time.Duration, now time.Time) ([]replyRow, error) {
	until := now.Add(-replyWithin * 24 * time.Hour)
	var rows []replyRow
	err := db.WithContext(ctx).Raw(fmt.Sprintf(replyCounts, column), map[string]any{
		"users": userIDs, "since": until.Add(-period), "until": until, "days": replyWithin,
	}).Scan(&rows).Error
	return rows, err
}

func replyRate(ctx context.Context, tx *gorm.DB, userID string, now time.Time) (float64, error) {
	rows, err := replies(ctx, tx, "candidate_user_id", []string{userID}, window, now)
	if err != nil || len(rows) == 0 {
		return noReplyData, err
	}
	return float64(rows[0].Replied) / float64(rows[0].Total), nil
}

// Ghosting returns which of these employers reply to too few of their
// matches (under 30% over 14 days, with at least 3 matches to judge).
func Ghosting(ctx context.Context, db *gorm.DB, employerIDs []string, now time.Time) (map[string]bool, error) {
	out := map[string]bool{}
	if len(employerIDs) == 0 {
		return out, nil
	}
	rows, err := replies(ctx, db, "employer_user_id", employerIDs, ghostWindow, now)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.Total >= ghostMinMatches && float64(r.Replied)/float64(r.Total) < ghostReplyRate {
			out[r.UserID] = true
		}
	}
	return out, nil
}
