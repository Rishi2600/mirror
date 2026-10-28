// Package hiring stores employers' hiring cards. One employer can have many
// cards (one per role); each is shown to fitting candidates once it is live.
package hiring

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"

	"mirror/server/internal/profiles"
)

const (
	StatusDraft  = "draft"
	StatusLive   = "live"
	StatusClosed = "closed"
)

const (
	minDescriptionChars = 50
	maxDescriptionChars = 20_000
)

var (
	ErrNotFound    = errors.New("hiring card not found")
	ErrDescription = errors.New("paste the job description (at least a few sentences)")
	ErrStatus      = errors.New("status must be live or closed")
)

var workModes = []string{"onsite", "hybrid", "remote"}

// Fields are the parts of a hiring card the AI fills in from the job
// description and the employer edits.
type Fields struct {
	Title       string   `json:"title"`
	CompanyName string   `json:"company_name"`
	Location    string   `json:"location"`
	WorkMode    string   `json:"work_mode"`
	Seniority   string   `json:"seniority"`
	Skills      []string `json:"skills"`
	Summary     string   `json:"summary"`
	// PayMin and PayMax are yearly, in PayCurrency; 0 means not given.
	PayMin          int    `json:"pay_min"`
	PayMax          int    `json:"pay_max"`
	PayCurrency     string `json:"pay_currency"`
	VisaSponsorship bool   `json:"visa_sponsorship"`
}

// Card is one row of hiring_cards.
type Card struct {
	ID              string `gorm:"type:uuid;default:gen_random_uuid()"`
	EmployerUserID  string
	Status          string `gorm:"default:draft"`
	Title           string
	CompanyName     string
	Location        string
	WorkMode        string
	Seniority       string
	Skills          []string `gorm:"serializer:json"`
	Summary         string
	PayMin          int
	PayMax          int
	PayCurrency     string
	VisaSponsorship bool
	Description     string
	ImageFileID     *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (Card) TableName() string { return "hiring_cards" }

// Fields returns the card's editable fields as one value.
func (c Card) Fields() Fields {
	return Fields{
		Title: c.Title, CompanyName: c.CompanyName, Location: c.Location,
		WorkMode: c.WorkMode, Seniority: c.Seniority, Skills: c.Skills, Summary: c.Summary,
		PayMin: c.PayMin, PayMax: c.PayMax, PayCurrency: c.PayCurrency, VisaSponsorship: c.VisaSponsorship,
	}
}

// JDParser turns a job description into card fields (the AI).
type JDParser interface {
	ParseJob(ctx context.Context, description string) (Fields, error)
}

// Create reads a pasted job description with the AI and saves it as a draft
// card for the employer to check and finish.
func Create(ctx context.Context, db *gorm.DB, parser JDParser, employerID, description string) (Card, error) {
	description = strings.TrimSpace(description)
	if len(description) < minDescriptionChars || len(description) > maxDescriptionChars {
		return Card{}, ErrDescription
	}
	f, err := parser.ParseJob(ctx, description)
	if err != nil {
		return Card{}, err
	}
	f = clean(f)
	// The AI may misread pay; keep only a range the database accepts.
	if f.PayMax > 0 && f.PayMax < f.PayMin {
		f.PayMin, f.PayMax = 0, 0
	}

	c := Card{EmployerUserID: employerID, Status: StatusDraft, Description: description}
	c.setFields(f)
	err = db.WithContext(ctx).Create(&c).Error
	return c, err
}

// List returns an employer's cards, newest first.
func List(ctx context.Context, db *gorm.DB, employerID string) ([]Card, error) {
	cards := []Card{}
	err := db.WithContext(ctx).Where("employer_user_id = ?", employerID).Order("created_at DESC").Find(&cards).Error
	return cards, err
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)

// Get returns one of the employer's own cards. Another employer's card is
// "not found", so card IDs reveal nothing.
func Get(ctx context.Context, db *gorm.DB, employerID, id string) (Card, error) {
	if !uuidPattern.MatchString(id) {
		return Card{}, ErrNotFound
	}
	var c Card
	err := db.WithContext(ctx).Where("id = ? AND employer_user_id = ?", id, employerID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Card{}, ErrNotFound
	}
	return c, err
}

// Update saves the employer's edits to a card.
func Update(ctx context.Context, db *gorm.DB, employerID, id string, f Fields) (Card, error) {
	c, err := Get(ctx, db, employerID, id)
	if err != nil {
		return Card{}, err
	}
	c.setFields(clean(f))
	err = db.WithContext(ctx).Model(&c).
		Select("title", "company_name", "location", "work_mode", "seniority", "skills", "summary",
			"pay_min", "pay_max", "pay_currency", "visa_sponsorship", "updated_at").
		Updates(&c).Error
	return c, err
}

// Validate checks the values the database would also reject, so the
// employer gets a clear message.
func Validate(f Fields) error {
	switch {
	case f.WorkMode != "" && !slices.Contains(workModes, f.WorkMode):
		return errors.New("work_mode must be onsite, hybrid or remote")
	case f.Seniority != "" && !slices.Contains(profiles.Seniorities, f.Seniority):
		return errors.New("seniority must be intern, junior, mid, senior, lead or principal")
	case f.PayMin < 0 || f.PayMax < 0:
		return errors.New("pay cannot be negative")
	case f.PayMax > 0 && f.PayMax < f.PayMin:
		return errors.New("the top of the pay range must not be below the bottom")
	case len(f.PayCurrency) != 3 || strings.ToUpper(f.PayCurrency) != f.PayCurrency:
		return errors.New("pay_currency must be a 3-letter code such as USD")
	}
	return nil
}

// Missing lists what a card needs before it can go live.
func Missing(c Card, firstName string) []string {
	var missing []string
	if c.Title == "" {
		missing = append(missing, "a job title")
	}
	if c.Summary == "" {
		missing = append(missing, "a short description")
	}
	if c.WorkMode == "" {
		missing = append(missing, "a work mode")
	}
	if strings.TrimSpace(firstName) == "" {
		missing = append(missing, "your first name")
	}
	return missing
}

// MissingError says what a card still needs to go live.
type MissingError struct{ Missing []string }

func (e MissingError) Error() string {
	return "this card still needs " + strings.Join(e.Missing, ", ")
}

// SetStatus makes a card live (shown to candidates) or closed (hidden).
func SetStatus(ctx context.Context, db *gorm.DB, employerID, id, status, firstName string) (Card, error) {
	if status != StatusLive && status != StatusClosed {
		return Card{}, ErrStatus
	}
	c, err := Get(ctx, db, employerID, id)
	if err != nil {
		return Card{}, err
	}
	if status == StatusLive {
		if missing := Missing(c, firstName); len(missing) > 0 {
			return Card{}, MissingError{Missing: missing}
		}
	}
	c.Status = status
	err = db.WithContext(ctx).Model(&c).Select("status", "updated_at").Updates(&c).Error
	return c, err
}

func (c *Card) setFields(f Fields) {
	c.Title, c.CompanyName, c.Location = f.Title, f.CompanyName, f.Location
	c.WorkMode, c.Seniority, c.Skills, c.Summary = f.WorkMode, f.Seniority, f.Skills, f.Summary
	c.PayMin, c.PayMax, c.PayCurrency, c.VisaSponsorship = f.PayMin, f.PayMax, f.PayCurrency, f.VisaSponsorship
}

// clean trims text, tidies skills and fills defaults the AI may leave out.
func clean(f Fields) Fields {
	trim := strings.TrimSpace
	f.Title, f.CompanyName, f.Location, f.Summary = trim(f.Title), trim(f.CompanyName), trim(f.Location), trim(f.Summary)
	f.WorkMode = strings.ToLower(trim(f.WorkMode))
	if !slices.Contains(workModes, f.WorkMode) {
		f.WorkMode = ""
	}
	f.Seniority = strings.ToLower(trim(f.Seniority))
	if !slices.Contains(profiles.Seniorities, f.Seniority) {
		f.Seniority = ""
	}
	f.Skills = profiles.CleanList(f.Skills)
	f.PayMin, f.PayMax = max(f.PayMin, 0), max(f.PayMax, 0)
	f.PayCurrency = strings.ToUpper(trim(f.PayCurrency))
	if len(f.PayCurrency) != 3 {
		f.PayCurrency = "USD"
	}
	return f
}
