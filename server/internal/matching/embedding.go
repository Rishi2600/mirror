package matching

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"mirror/server/internal/embed"
	"mirror/server/internal/hiring"
	"mirror/server/internal/profiles"
)

const (
	OwnerCandidate  = "candidate"
	OwnerHiringCard = "hiring_card"
)

// CandidateText is the text a candidate card is embedded as: the work only,
// from the scrubbed card. Names, contacts, pay and location are left out
// (paper, section 5).
func CandidateText(p profiles.Profile) string {
	card, _ := profiles.BuildCard(p, "")
	var b strings.Builder
	line := func(parts ...string) {
		text := strings.TrimSpace(strings.Join(parts, " "))
		if text != "" {
			b.WriteString(text)
			b.WriteString("\n")
		}
	}
	line(card.Headline+".", card.CurrentTitle+".")
	if card.Seniority != "" || card.YearsExperience > 0 {
		line(fmt.Sprintf("%s, %d years.", card.Seniority, card.YearsExperience))
	}
	if len(card.Skills) > 0 {
		line("Skills:", strings.Join(card.Skills, ", ")+".")
	}
	line(card.Summary)
	var roles []string
	for _, job := range card.Experience {
		roles = append(roles, job.Title)
	}
	if len(roles) > 0 {
		line("Recent roles:", strings.Join(roles, "; ")+".")
	}
	if len(card.RoleTypes) > 0 {
		line("Looking for:", strings.Join(card.RoleTypes, ", ")+".")
	}
	return strings.TrimSpace(b.String())
}

// JobText is the text a hiring card is embedded as: the role only, from the
// scrubbed view.
func JobText(c hiring.Card) string {
	view, _ := hiring.BuildView(c, "", "")
	var parts []string
	parts = append(parts, strings.TrimSpace(view.Title+". "+view.Seniority+"."))
	if len(view.Skills) > 0 {
		parts = append(parts, "Skills: "+strings.Join(view.Skills, ", ")+".")
	}
	parts = append(parts, view.Summary)
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// EmbedOwner makes or refreshes the vector of one card. Text that has not
// changed since the last vector is not embedded again. Cards that are not
// live (draft or closed hiring cards, unfinished candidate cards) are
// skipped; the timelines never show them anyway.
func EmbedOwner(ctx context.Context, db *gorm.DB, embedder embed.Embedder, ownerType, ownerID string) error {
	text, ok, err := ownerText(ctx, db, ownerType, ownerID)
	if err != nil || !ok {
		return err
	}
	hash := hashText(text)

	var current string
	db.WithContext(ctx).Raw(`SELECT text_hash FROM embeddings WHERE owner_type = ? AND owner_id = ? AND model = ?`,
		ownerType, ownerID, embed.Model).Scan(&current)
	if current == hash {
		return nil
	}

	vectors, err := embedder.Embed(ctx, []string{embed.Prefix(text)})
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Exec(`
		INSERT INTO embeddings (owner_type, owner_id, model, text_hash, embedding, updated_at)
		VALUES (?, ?, ?, ?, ?::halfvec, now())
		ON CONFLICT (owner_type, owner_id, model)
		DO UPDATE SET text_hash = EXCLUDED.text_hash, embedding = EXCLUDED.embedding, updated_at = now()`,
		ownerType, ownerID, embed.Model, hash, embed.Literal(vectors[0])).Error
}

// ownerText loads a card and returns its text, or false when it is not live.
func ownerText(ctx context.Context, db *gorm.DB, ownerType, ownerID string) (string, bool, error) {
	switch ownerType {
	case OwnerCandidate:
		p, err := profiles.Get(ctx, db, ownerID)
		if errors.Is(err, profiles.ErrNotFound) {
			return "", false, nil
		}
		if err != nil || p.OnboardedAt == nil {
			return "", false, err
		}
		return CandidateText(p), true, nil
	case OwnerHiringCard:
		var c hiring.Card
		err := db.WithContext(ctx).Where("id = ?", ownerID).First(&c).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		if err != nil || c.Status != hiring.StatusLive {
			return "", false, err
		}
		return JobText(c), true, nil
	}
	return "", false, fmt.Errorf("unknown owner type %q", ownerType)
}

// Owner names one card that needs a vector.
type Owner struct {
	Type string
	ID   string
}

// MissingEmbeddings lists live cards with no vector from the current model
// (new cards, and every card after a model change).
func MissingEmbeddings(ctx context.Context, db *gorm.DB) ([]Owner, error) {
	var owners []Owner
	err := db.WithContext(ctx).Raw(`
		SELECT 'candidate' AS type, p.user_id::text AS id
		FROM candidate_profiles p
		WHERE p.onboarded_at IS NOT NULL AND NOT EXISTS (
			SELECT 1 FROM embeddings e
			WHERE e.owner_type = 'candidate' AND e.owner_id = p.user_id AND e.model = ?)
		UNION ALL
		SELECT 'hiring_card', h.id::text
		FROM hiring_cards h
		WHERE h.status = 'live' AND NOT EXISTS (
			SELECT 1 FROM embeddings e
			WHERE e.owner_type = 'hiring_card' AND e.owner_id = h.id AND e.model = ?)`,
		embed.Model, embed.Model).Scan(&owners).Error
	return owners, err
}
