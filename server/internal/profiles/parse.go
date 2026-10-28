package profiles

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"gorm.io/gorm"

	"mirror/server/internal/cvtext"
	"mirror/server/internal/storage"
)

// CVParser turns CV text into a structured CV (the AI).
type CVParser interface {
	ParseCV(ctx context.Context, text string) (CV, error)
}

// maxTextChars caps the CV text sent to the AI.
const maxTextChars = 20_000

// ParseCV reads the CV file and fills the profile from it. It does nothing
// if the candidate has uploaded a newer CV since. A file with no readable
// text marks the profile failed and returns nil (retrying cannot help); an
// AI or network error is returned, so the job can retry.
func ParseCV(ctx context.Context, db *gorm.DB, files *storage.Local, parser CVParser, userID, fileID string) error {
	p, err := Get(ctx, db, userID)
	if err != nil {
		return err
	}
	if p.CVFileID == nil || *p.CVFileID != fileID {
		return nil
	}

	var file File
	if err := db.WithContext(ctx).Where("id = ?", fileID).First(&file).Error; err != nil {
		return err
	}
	data, err := files.Open(file.StorageKey)
	if err != nil {
		return err
	}
	text, err := cvtext.Extract(data)
	if errors.Is(err, cvtext.ErrNoText) || errors.Is(err, cvtext.ErrUnsupported) {
		return MarkFailed(ctx, db, userID, fileID, err.Error())
	}
	if err != nil {
		return MarkFailed(ctx, db, userID, fileID, "the CV file could not be read; try saving it again as PDF")
	}
	if len(text) > maxTextChars {
		text = text[:maxTextChars]
	}

	cv, err := parser.ParseCV(ctx, text)
	if err != nil {
		return fmt.Errorf("parsing the CV with AI: %w", err)
	}

	// The current employer is hidden from the card by default.
	cv = cleanCV(cv)
	hiddenFrom := CleanList(append(p.HiddenFrom, cv.CurrentEmployer))

	// Only update if this file is still the current CV. A struct update (not
	// a map) so GORM stores JSON columns; Select includes the empty parse_error.
	return db.WithContext(ctx).Model(&Profile{}).
		Where("user_id = ? AND cv_file_id = ?", userID, fileID).
		Select("cv", "hidden_from", "parse_status", "parse_error", "updated_at").
		Updates(&Profile{CV: cv, HiddenFrom: hiddenFrom, ParseStatus: StatusDone}).Error
}

// MarkFailed records that the CV could not be read, with a message for the
// candidate.
func MarkFailed(ctx context.Context, db *gorm.DB, userID, fileID, message string) error {
	return db.WithContext(ctx).Model(&Profile{}).
		Where("user_id = ? AND cv_file_id = ?", userID, fileID).
		Updates(map[string]any{"parse_status": StatusFailed, "parse_error": message}).Error
}

// ValidatePreferences checks the values the database would also reject, so
// the candidate gets a clear message.
func ValidatePreferences(p Preferences) error {
	switch {
	case !slices.Contains([]string{"onsite", "hybrid", "remote", "any"}, p.WorkMode):
		return errors.New("work_mode must be onsite, hybrid, remote or any")
	case p.MinPay < 0:
		return errors.New("min_pay cannot be negative")
	case len(p.PayCurrency) != 3 || strings.ToUpper(p.PayCurrency) != p.PayCurrency:
		return errors.New("pay_currency must be a 3-letter code such as USD")
	}
	return nil
}

// ValidateCardSettings checks the card choices.
func ValidateCardSettings(c CardSettings) error {
	if !slices.Contains([]string{"name", "description", "hidden"}, c.EmployerDisplay) {
		return errors.New("employer_display must be name, description or hidden")
	}
	if c.EmployerDisplay == "description" && strings.TrimSpace(c.EmployerDescription) == "" {
		return errors.New("describe your current employer, or pick another option")
	}
	return nil
}

// cleanCV trims text, drops empty and repeated skills, and keeps seniority
// to the allowed values.
func cleanCV(cv CV) CV {
	trim := strings.TrimSpace
	cv.FullName, cv.Email, cv.Phone = trim(cv.FullName), trim(cv.Email), trim(cv.Phone)
	cv.Headline, cv.Summary = trim(cv.Headline), trim(cv.Summary)
	cv.CurrentTitle, cv.CurrentEmployer, cv.Location = trim(cv.CurrentTitle), trim(cv.CurrentEmployer), trim(cv.Location)
	cv.Seniority = strings.ToLower(trim(cv.Seniority))
	if !slices.Contains(Seniorities, cv.Seniority) {
		cv.Seniority = ""
	}
	cv.YearsExperience = max(cv.YearsExperience, 0)
	cv.Skills = CleanList(cv.Skills)
	if cv.Experience == nil {
		cv.Experience = []Experience{}
	}
	for i, job := range cv.Experience {
		cv.Experience[i] = Experience{
			Title: trim(job.Title), Company: trim(job.Company),
			Start: trim(job.Start), End: trim(job.End), Summary: trim(job.Summary),
		}
	}
	if cv.Education == nil {
		cv.Education = []Education{}
	}
	return cv
}

// CleanList trims items and drops empty and repeated ones (ignoring case).
func CleanList(items []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		key := strings.ToLower(item)
		if item == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}
