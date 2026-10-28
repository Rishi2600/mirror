package hiring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"gorm.io/gorm"

	"mirror/server/internal/photos"
	"mirror/server/internal/profiles"
	"mirror/server/internal/scrub"
	"mirror/server/internal/storage"
)

var ErrNoImage = errors.New("no image yet")

// View is a hiring card as candidates see it before a match: the role, the
// employer's first name and one image. Contact details stay hidden (the
// text is scrubbed of emails, phones and links).
type View struct {
	Title             string   `json:"title"`
	CompanyName       string   `json:"company_name"`
	EmployerFirstName string   `json:"employer_first_name"`
	ImageURL          string   `json:"image_url"`
	Location          string   `json:"location"`
	WorkMode          string   `json:"work_mode"`
	Seniority         string   `json:"seniority"`
	Skills            []string `json:"skills"`
	Summary           string   `json:"summary"`
	PayMin            int      `json:"pay_min"`
	PayMax            int      `json:"pay_max"`
	PayCurrency       string   `json:"pay_currency"`
	VisaSponsorship   bool     `json:"visa_sponsorship"`
}

// BuildView turns a card into what candidates see. imageURL is where the
// viewer loads the image ("" when there is none).
func BuildView(c Card, firstName, imageURL string) (View, scrub.Removed) {
	var removed scrub.Removed
	clean := func(s string) string {
		out, r := scrub.Text(s)
		removed = removed.Add(r)
		return out
	}
	return View{
		Title:             clean(c.Title),
		CompanyName:       clean(c.CompanyName),
		EmployerFirstName: clean(firstName),
		ImageURL:          imageURL,
		Location:          clean(c.Location),
		WorkMode:          c.WorkMode,
		Seniority:         c.Seniority,
		Skills:            c.Skills,
		Summary:           clean(c.Summary),
		PayMin:            c.PayMin,
		PayMax:            c.PayMax,
		PayCurrency:       c.PayCurrency,
		VisaSponsorship:   c.VisaSponsorship,
	}, removed
}

// SetImage stores a card's image (a logo or the employer's photo), with
// metadata removed, and deletes the old one.
func SetImage(ctx context.Context, db *gorm.DB, files *storage.Local, employerID, id string, data []byte) (Card, error) {
	c, err := Get(ctx, db, employerID, id)
	if err != nil {
		return Card{}, err
	}
	jpg, err := photos.Prepare(data)
	if err != nil {
		return Card{}, err
	}
	key, err := files.Save(jpg)
	if err != nil {
		return Card{}, err
	}
	sum := sha256.Sum256(jpg)
	file := profiles.File{
		OwnerUserID: employerID,
		Kind:        "card_image",
		StorageKey:  key,
		ContentType: "image/jpeg",
		SizeBytes:   int64(len(jpg)),
		Sha256:      hex.EncodeToString(sum[:]),
	}

	oldID := c.ImageFileID
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&file).Error; err != nil {
			return err
		}
		c.ImageFileID = &file.ID
		return tx.Model(&c).Select("image_file_id", "updated_at").Updates(&c).Error
	})
	if err != nil {
		files.Delete(key)
		return Card{}, err
	}

	if oldID != nil {
		var old profiles.File
		if db.WithContext(ctx).Where("id = ?", *oldID).First(&old).Error == nil {
			db.WithContext(ctx).Delete(&old)
			files.Delete(old.StorageKey)
		}
	}
	return c, nil
}

// Image returns a card's image as JPEG bytes.
func Image(ctx context.Context, db *gorm.DB, files *storage.Local, c Card) ([]byte, error) {
	if c.ImageFileID == nil {
		return nil, ErrNoImage
	}
	var file profiles.File
	if err := db.WithContext(ctx).Where("id = ?", *c.ImageFileID).First(&file).Error; err != nil {
		return nil, err
	}
	return files.Open(file.StorageKey)
}
