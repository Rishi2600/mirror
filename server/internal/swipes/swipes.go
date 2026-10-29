// Package swipes records swipes and creates matches. A match is created in
// the same transaction as the second right swipe on a pair, with its reveal
// audit event, so a match never exists without its record (or twice).
package swipes

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"mirror/server/internal/hiring"
	"mirror/server/internal/profiles"
	"mirror/server/internal/users"
)

const (
	SideCandidate = "candidate"
	SideEmployer  = "employer"
	Left          = "left"
	Right         = "right"

	StateMatched   = "matched"
	StateUnmatched = "unmatched"

	maxKeyLength = 100
)

var (
	ErrDirection     = errors.New("direction must be left or right")
	ErrKey           = errors.New("send an Idempotency-Key header (1 to 100 characters)")
	ErrKeyReused     = errors.New("this Idempotency-Key was already used for a different swipe")
	ErrAlreadySwiped = errors.New("you have already swiped on this one")
	ErrNotAllowed    = errors.New("this card cannot be swiped on")
	ErrMatchNotFound = errors.New("match not found")
)

type Swipe struct {
	ID              string `gorm:"type:uuid;default:gen_random_uuid()"`
	Side            string
	ActorUserID     string
	HiringCardID    string
	CandidateUserID string
	Direction       string
	IdempotencyKey  string
	CreatedAt       time.Time
}

type Match struct {
	ID              string `gorm:"type:uuid;default:gen_random_uuid()"`
	HiringCardID    string
	CandidateUserID string
	EmployerUserID  string
	State           string `gorm:"default:matched"`
	MatchedAt       time.Time
	UnmatchedAt     *time.Time
	UnmatchedBy     *string
	// When each side last read the chat; nil until they open the match.
	CandidateReadAt *time.Time
	EmployerReadAt  *time.Time
}

// OtherUserID returns the other person in the match.
func (m Match) OtherUserID(userID string) string {
	if userID == m.CandidateUserID {
		return m.EmployerUserID
	}
	return m.CandidateUserID
}

// ReadAt returns when this user last read the chat, and when the other
// person did.
func (m Match) ReadAt(userID string) (mine, theirs *time.Time) {
	if userID == m.CandidateUserID {
		return m.CandidateReadAt, m.EmployerReadAt
	}
	return m.EmployerReadAt, m.CandidateReadAt
}

type auditEvent struct {
	ActorUserID string
	Action      string
	MatchID     string
	CreatedAt   time.Time
}

func (auditEvent) TableName() string { return "audit_events" }

// Result is a recorded swipe, and the match when it made one.
type Result struct {
	Swipe Swipe
	Match *Match
}

// Record saves one swipe by actor on the pair (hiring card, candidate). A
// candidate swipes on a live hiring card for themselves; an employer swipes
// on a finished candidate card for one of their own live hiring cards.
// Sending the same key again returns the first result.
func Record(ctx context.Context, db *gorm.DB, actor users.User, cardID, candidateID, direction, key string) (Result, error) {
	if direction != Left && direction != Right {
		return Result{}, ErrDirection
	}
	if key == "" || len(key) > maxKeyLength {
		return Result{}, ErrKey
	}
	side, otherSide := SideCandidate, SideEmployer
	if actor.Role == users.RoleEmployer {
		side, otherSide = SideEmployer, SideCandidate
	}

	var res Result
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var prev Swipe
		err := tx.Where("actor_user_id = ? AND idempotency_key = ?", actor.ID, key).First(&prev).Error
		if err == nil {
			if prev.HiringCardID != cardID || prev.CandidateUserID != candidateID || prev.Direction != direction {
				return ErrKeyReused
			}
			res.Swipe = prev
			res.Match = findMatch(tx, cardID, candidateID)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		card, err := swipeable(tx, actor, cardID, candidateID)
		if err != nil {
			return err
		}

		// Lock this pair for the rest of the transaction, so two swipes on it
		// run one after the other and the second always sees the first.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", cardID+":"+candidateID).Error; err != nil {
			return err
		}

		s := Swipe{Side: side, ActorUserID: actor.ID, HiringCardID: cardID, CandidateUserID: candidateID, Direction: direction, IdempotencyKey: key}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&s)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			return ErrAlreadySwiped
		}
		res.Swipe = s

		if direction != Right {
			return nil
		}
		var liked int64
		err = tx.Model(&Swipe{}).
			Where("side = ? AND hiring_card_id = ? AND candidate_user_id = ? AND direction = ?", otherSide, cardID, candidateID, Right).
			Count(&liked).Error
		if err != nil || liked == 0 {
			return err
		}

		m := Match{HiringCardID: cardID, CandidateUserID: candidateID, EmployerUserID: card.EmployerUserID}
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
		if err := tx.Create(&auditEvent{ActorUserID: actor.ID, Action: "reveal", MatchID: m.ID}).Error; err != nil {
			return err
		}
		res.Match = &m
		return nil
	})
	return res, err
}

// swipeable checks that actor may swipe on this pair, and returns the card.
func swipeable(tx *gorm.DB, actor users.User, cardID, candidateID string) (hiring.Card, error) {
	var card hiring.Card
	if err := tx.Where("id = ? AND status = ?", cardID, hiring.StatusLive).First(&card).Error; err != nil {
		return hiring.Card{}, ErrNotAllowed
	}
	switch actor.Role {
	case users.RoleCandidate:
		if candidateID != actor.ID {
			return hiring.Card{}, ErrNotAllowed
		}
	case users.RoleEmployer:
		if card.EmployerUserID != actor.ID {
			return hiring.Card{}, ErrNotAllowed
		}
	default:
		return hiring.Card{}, ErrNotAllowed
	}
	var p profiles.Profile
	if err := tx.Where("user_id = ? AND onboarded_at IS NOT NULL", candidateID).First(&p).Error; err != nil {
		return hiring.Card{}, ErrNotAllowed
	}
	return card, nil
}

func findMatch(tx *gorm.DB, cardID, candidateID string) *Match {
	var m Match
	if tx.Where("hiring_card_id = ? AND candidate_user_id = ?", cardID, candidateID).First(&m).Error != nil {
		return nil
	}
	return &m
}

// Matches lists a user's live matches, newest first.
func Matches(ctx context.Context, db *gorm.DB, user users.User) ([]Match, error) {
	matches := []Match{}
	q := db.WithContext(ctx).Where("state = ?", StateMatched).Order("matched_at DESC")
	if user.Role == users.RoleEmployer {
		q = q.Where("employer_user_id = ?", user.ID)
	} else {
		q = q.Where("candidate_user_id = ?", user.ID)
	}
	err := q.Find(&matches).Error
	return matches, err
}

// Get returns one of the user's live matches. Anyone else's match, or one
// that was ended, is not found.
func Get(ctx context.Context, db *gorm.DB, user users.User, id string) (Match, error) {
	var m Match
	err := db.WithContext(ctx).
		Where("id::text = ? AND state = ? AND (candidate_user_id = ? OR employer_user_id = ?)", id, StateMatched, user.ID, user.ID).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Match{}, ErrMatchNotFound
	}
	return m, err
}

// Unmatch ends a match; either side may. The employer loses the contact
// details and CV at once, and the chat closes for both.
func Unmatch(ctx context.Context, db *gorm.DB, user users.User, id string) (Match, error) {
	var m Match
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("id::text = ? AND state = ? AND (candidate_user_id = ? OR employer_user_id = ?)", id, StateMatched, user.ID, user.ID).
			First(&m).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMatchNotFound
		}
		if err != nil {
			return err
		}
		res := tx.Model(&Match{}).Where("id = ? AND state = ?", m.ID, StateMatched).
			Updates(map[string]any{"state": StateUnmatched, "unmatched_at": time.Now(), "unmatched_by": user.ID})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrMatchNotFound // ended by the other side a moment ago
		}
		return tx.Create(&auditEvent{ActorUserID: user.ID, Action: "unmatch", MatchID: m.ID}).Error
	})
	return m, err
}

// Audit records that a user saw or downloaded something from a match.
func Audit(ctx context.Context, db *gorm.DB, actorID, action, matchID string) error {
	return db.WithContext(ctx).Create(&auditEvent{ActorUserID: actorID, Action: action, MatchID: matchID}).Error
}
