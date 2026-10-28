// Package profiles stores candidate profiles: the uploaded CV, what the AI
// read from it, and the candidate's job preferences.
package profiles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"mirror/server/internal/cvtext"
	"mirror/server/internal/storage"
)

const (
	StatusPending = "pending"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

var (
	ErrNotFound = errors.New("no profile yet; upload a CV first")
	ErrParsing  = errors.New("the CV is still being read; try again in a moment")
)

// CV is what the AI read from the CV. The candidate can edit all of it.
type CV struct {
	FullName        string       `json:"full_name"`
	Email           string       `json:"email"`
	Phone           string       `json:"phone"`
	Headline        string       `json:"headline"`
	Summary         string       `json:"summary"`
	CurrentTitle    string       `json:"current_title"`
	CurrentEmployer string       `json:"current_employer"`
	Location        string       `json:"location"`
	YearsExperience int          `json:"years_experience"`
	Seniority       string       `json:"seniority"`
	Skills          []string     `json:"skills"`
	Experience      []Experience `json:"experience"`
	Education       []Education  `json:"education"`
}

type Experience struct {
	Title   string `json:"title"`
	Company string `json:"company"`
	Start   string `json:"start"`
	End     string `json:"end"`
	Summary string `json:"summary"`
}

type Education struct {
	Degree      string `json:"degree"`
	Institution string `json:"institution"`
	Year        string `json:"year"`
}

// Seniorities are the allowed CV.Seniority values ("" means unknown).
var Seniorities = []string{"intern", "junior", "mid", "senior", "lead", "principal"}

// Preferences are what the candidate is looking for.
type Preferences struct {
	RoleTypes   []string `json:"role_types"`
	Locations   []string `json:"locations"`
	WorkMode    string   `json:"work_mode"`
	MinPay      int      `json:"min_pay"`
	PayCurrency string   `json:"pay_currency"`
	NeedsVisa   bool     `json:"needs_visa"`
	// HiddenFrom lists companies that must never see the card.
	HiddenFrom []string `json:"hidden_from"`
}

// CardSettings are the candidate's choices about what the card shows.
type CardSettings struct {
	// EmployerDisplay is name, description or hidden.
	EmployerDisplay     string `json:"employer_display"`
	EmployerDescription string `json:"employer_description"`
	ShowPay             bool   `json:"show_pay"`
}

// Profile is one row of candidate_profiles.
type Profile struct {
	UserID      string `gorm:"primaryKey"`
	CVFileID    *string
	ParseStatus string
	ParseError  string
	CV          CV       `gorm:"serializer:json"`
	RoleTypes   []string `gorm:"serializer:json"`
	Locations   []string `gorm:"serializer:json"`
	WorkMode    string
	MinPay      int
	PayCurrency string
	NeedsVisa   bool
	HiddenFrom  []string `gorm:"serializer:json"`

	PhotoFileID         *string
	EmployerDisplay     string
	EmployerDescription string
	ShowPay             bool
	// OnboardedAt is set when the card is finished; the card is live from then.
	OnboardedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Profile) TableName() string { return "candidate_profiles" }

// CardSettings returns the profile's card choices as one value.
func (p Profile) CardSettings() CardSettings {
	return CardSettings{
		EmployerDisplay:     p.EmployerDisplay,
		EmployerDescription: p.EmployerDescription,
		ShowPay:             p.ShowPay,
	}
}

// Preferences returns the profile's preference columns as one value.
func (p Profile) Preferences() Preferences {
	return Preferences{
		RoleTypes:   p.RoleTypes,
		Locations:   p.Locations,
		WorkMode:    p.WorkMode,
		MinPay:      p.MinPay,
		PayCurrency: p.PayCurrency,
		NeedsVisa:   p.NeedsVisa,
		HiddenFrom:  p.HiddenFrom,
	}
}

// File is one row of the files table.
type File struct {
	ID          string `gorm:"type:uuid;default:gen_random_uuid()"`
	OwnerUserID string
	Kind        string
	StorageKey  string
	ContentType string
	SizeBytes   int64
	Sha256      string `gorm:"column:sha256"`
	CreatedAt   time.Time
}

// Get returns a candidate's profile.
func Get(ctx context.Context, db *gorm.DB, userID string) (Profile, error) {
	var p Profile
	err := db.WithContext(ctx).Where("user_id = ?", userID).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Profile{}, ErrNotFound
	}
	return p, err
}

// UploadCV stores a new CV and marks the profile as waiting to be parsed.
// It keeps the earlier CV fields and preferences, and deletes the old file.
// The caller then queues the parse job for the returned file ID.
func UploadCV(ctx context.Context, db *gorm.DB, files *storage.Local, userID string, data []byte) (Profile, string, error) {
	contentType, err := cvtext.DetectType(data)
	if err != nil {
		return Profile{}, "", err
	}
	key, err := files.Save(data)
	if err != nil {
		return Profile{}, "", err
	}
	sum := sha256.Sum256(data)
	file := File{
		OwnerUserID: userID,
		Kind:        "cv",
		StorageKey:  key,
		ContentType: contentType,
		SizeBytes:   int64(len(data)),
		Sha256:      hex.EncodeToString(sum[:]),
	}

	var old *File
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&file).Error; err != nil {
			return err
		}
		var existing Profile
		if err := tx.Where("user_id = ?", userID).First(&existing).Error; err == nil && existing.CVFileID != nil {
			old = &File{}
			if err := tx.Where("id = ?", *existing.CVFileID).First(old).Error; err != nil {
				old = nil
			}
		}

		p := newProfile(userID)
		p.CVFileID = &file.ID
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"cv_file_id", "parse_status", "parse_error", "updated_at"}),
		}).Create(&p).Error
	})
	if err != nil {
		files.Delete(key)
		return Profile{}, "", err
	}

	if old != nil {
		db.WithContext(ctx).Delete(&File{}, "id = ?", old.ID)
		files.Delete(old.StorageKey)
	}
	p, err := Get(ctx, db, userID)
	return p, file.ID, err
}

// newProfile is an empty pending profile with default preferences.
func newProfile(userID string) Profile {
	return Profile{
		UserID:          userID,
		ParseStatus:     StatusPending,
		CV:              CV{Skills: []string{}, Experience: []Experience{}, Education: []Education{}},
		RoleTypes:       []string{},
		Locations:       []string{},
		WorkMode:        "any",
		PayCurrency:     "USD",
		HiddenFrom:      []string{},
		EmployerDisplay: "name",
		ShowPay:         true,
	}
}

// Update saves the candidate's edits. It is refused while a CV is being read,
// because the parse result would overwrite the edits.
func Update(ctx context.Context, db *gorm.DB, userID string, cv CV, prefs Preferences, settings CardSettings) (Profile, error) {
	p, err := Get(ctx, db, userID)
	if err != nil {
		return Profile{}, err
	}
	if p.ParseStatus == StatusPending {
		return Profile{}, ErrParsing
	}

	p.CV = cleanCV(cv)
	p.RoleTypes = CleanList(prefs.RoleTypes)
	p.Locations = CleanList(prefs.Locations)
	p.WorkMode = prefs.WorkMode
	p.MinPay = prefs.MinPay
	p.PayCurrency = prefs.PayCurrency
	p.NeedsVisa = prefs.NeedsVisa
	p.HiddenFrom = CleanList(prefs.HiddenFrom)
	p.EmployerDisplay = settings.EmployerDisplay
	p.EmployerDescription = strings.TrimSpace(settings.EmployerDescription)
	p.ShowPay = settings.ShowPay
	err = db.WithContext(ctx).Model(&p).
		Select("cv", "role_types", "locations", "work_mode", "min_pay", "pay_currency", "needs_visa",
			"hidden_from", "employer_display", "employer_description", "show_pay", "updated_at").
		Updates(&p).Error
	return p, err
}
