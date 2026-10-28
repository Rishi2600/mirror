package profiles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"mirror/server/internal/photos"
	"mirror/server/internal/scrub"
	"mirror/server/internal/storage"
)

var ErrNoPhoto = errors.New("no photo yet")

// Card is what employers see before a match: the name, one photo and the
// professional details. It has no fields for email, phone, links or the CV
// file, and its free text is scrubbed of them.
type Card struct {
	Name            string           `json:"name"`
	PhotoURL        string           `json:"photo_url"`
	Headline        string           `json:"headline"`
	Location        string           `json:"location"`
	CurrentTitle    string           `json:"current_title"`
	CurrentEmployer string           `json:"current_employer"`
	YearsExperience int              `json:"years_experience"`
	Seniority       string           `json:"seniority"`
	Skills          []string         `json:"skills"`
	Summary         string           `json:"summary"`
	Experience      []CardExperience `json:"experience"`
	Education       []Education      `json:"education"`
	WorkMode        string           `json:"work_mode"`
	RoleTypes       []string         `json:"role_types"`
	// MinPay is 0 when the candidate hides it.
	MinPay      int    `json:"min_pay"`
	PayCurrency string `json:"pay_currency"`
}

// CardExperience is one job on the card. Company follows the candidate's
// choice for the current employer.
type CardExperience struct {
	Title   string `json:"title"`
	Company string `json:"company"`
	Start   string `json:"start"`
	End     string `json:"end"`
	Summary string `json:"summary"`
}

// BuildCard turns a profile into its card. photoURL is where the viewer can
// load the photo ("" when there is none). It also returns what the scrub
// removed, so the candidate can be told.
func BuildCard(p Profile, photoURL string) (Card, scrub.Removed) {
	var removed scrub.Removed
	clean := func(s string) string {
		out, r := scrub.Text(s)
		removed = removed.Add(r)
		return out
	}

	cv := p.CV
	card := Card{
		Name:            clean(cv.FullName),
		PhotoURL:        photoURL,
		Headline:        clean(cv.Headline),
		Location:        clean(cv.Location),
		CurrentTitle:    clean(cv.CurrentTitle),
		CurrentEmployer: p.shownEmployer(cv.CurrentEmployer),
		YearsExperience: cv.YearsExperience,
		Seniority:       cv.Seniority,
		Skills:          cv.Skills,
		Summary:         clean(cv.Summary),
		Experience:      []CardExperience{},
		Education:       []Education{},
		WorkMode:        p.WorkMode,
		RoleTypes:       p.RoleTypes,
		PayCurrency:     p.PayCurrency,
	}
	if p.ShowPay {
		card.MinPay = p.MinPay
	}
	for _, job := range cv.Experience {
		company := clean(job.Company)
		if strings.EqualFold(job.Company, cv.CurrentEmployer) {
			company = card.CurrentEmployer
		}
		card.Experience = append(card.Experience, CardExperience{
			Title:   clean(job.Title),
			Company: company,
			Start:   job.Start,
			End:     job.End,
			Summary: clean(job.Summary),
		})
	}
	for _, ed := range cv.Education {
		card.Education = append(card.Education, Education{
			Degree: clean(ed.Degree), Institution: clean(ed.Institution), Year: ed.Year,
		})
	}
	return card, removed
}

// shownEmployer applies the candidate's choice for the current employer.
func (p Profile) shownEmployer(name string) string {
	switch p.EmployerDisplay {
	case "description":
		out, _ := scrub.Text(p.EmployerDescription)
		return out
	case "hidden":
		return ""
	}
	out, _ := scrub.Text(name)
	return out
}

// Missing lists what the card still needs before the candidate can finish.
func Missing(p Profile) []string {
	var missing []string
	if p.ParseStatus != StatusDone {
		missing = append(missing, "a CV we have read")
	}
	if p.PhotoFileID == nil {
		missing = append(missing, "a photo")
	}
	if strings.TrimSpace(p.CV.FullName) == "" {
		missing = append(missing, "your name")
	}
	if strings.TrimSpace(p.CV.Headline) == "" {
		missing = append(missing, "a headline")
	}
	return missing
}

// MissingError says what the card still needs.
type MissingError struct{ Missing []string }

func (e MissingError) Error() string {
	return "your card still needs " + strings.Join(e.Missing, ", ")
}

// Finish marks the card as built, which makes it live for employers.
// Finishing again does nothing.
func Finish(ctx context.Context, db *gorm.DB, userID string) (Profile, error) {
	p, err := Get(ctx, db, userID)
	if err != nil {
		return Profile{}, err
	}
	if missing := Missing(p); len(missing) > 0 {
		return Profile{}, MissingError{Missing: missing}
	}
	if p.OnboardedAt == nil {
		now := time.Now()
		p.OnboardedAt = &now
		err = db.WithContext(ctx).Model(&p).Select("onboarded_at").Updates(&p).Error
	}
	return p, err
}

// SetPhoto stores a new profile photo (metadata removed, shrunk) and deletes
// the old one.
func SetPhoto(ctx context.Context, db *gorm.DB, files *storage.Local, userID string, data []byte) (Profile, error) {
	p, err := Get(ctx, db, userID)
	if err != nil {
		return Profile{}, err
	}
	jpg, err := photos.Prepare(data)
	if err != nil {
		return Profile{}, err
	}
	key, err := files.Save(jpg)
	if err != nil {
		return Profile{}, err
	}
	sum := sha256.Sum256(jpg)
	file := File{
		OwnerUserID: userID,
		Kind:        "photo",
		StorageKey:  key,
		ContentType: "image/jpeg",
		SizeBytes:   int64(len(jpg)),
		Sha256:      hex.EncodeToString(sum[:]),
	}

	oldID := p.PhotoFileID
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&file).Error; err != nil {
			return err
		}
		p.PhotoFileID = &file.ID
		return tx.Model(&p).Select("photo_file_id", "updated_at").Updates(&p).Error
	})
	if err != nil {
		files.Delete(key)
		return Profile{}, err
	}

	if oldID != nil {
		var old File
		if db.WithContext(ctx).Where("id = ?", *oldID).First(&old).Error == nil {
			db.WithContext(ctx).Delete(&old)
			files.Delete(old.StorageKey)
		}
	}
	return p, nil
}

// Photo returns a candidate's photo as JPEG bytes.
func Photo(ctx context.Context, db *gorm.DB, files *storage.Local, userID string) ([]byte, error) {
	p, err := Get(ctx, db, userID)
	if err != nil {
		return nil, err
	}
	if p.PhotoFileID == nil {
		return nil, ErrNoPhoto
	}
	var file File
	if err := db.WithContext(ctx).Where("id = ?", *p.PhotoFileID).First(&file).Error; err != nil {
		return nil, err
	}
	return files.Open(file.StorageKey)
}
