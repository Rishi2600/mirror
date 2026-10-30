// Package users stores user accounts.
package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Role string

const (
	RoleCandidate Role = "candidate"
	RoleEmployer  Role = "employer"
	RoleAdmin     Role = "admin"
)

type User struct {
	ID    string `gorm:"type:uuid;default:gen_random_uuid()"`
	Email string
	// Role is empty until the user picks one (stored as NULL).
	Role Role `gorm:"default:null"`
	// FirstName is shown on an employer's hiring cards.
	FirstName string
	// TimeZone is the browser's zone, saved at sign-in; empty means UTC.
	TimeZone  string
	CreatedAt time.Time
}

var (
	ErrEmailTaken = errors.New("email is already registered")
	ErrNotFound   = errors.New("user not found")
	ErrRoleSet    = errors.New("role is already set")
)

// Create adds a user. The email is stored trimmed and lowercase. An empty
// role means the user has not picked one yet.
func Create(ctx context.Context, db *gorm.DB, email string, role Role) (User, error) {
	u := User{Email: NormalizeEmail(email), Role: role}
	err := db.WithContext(ctx).Create(&u).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return User{}, ErrEmailTaken
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// ByEmail finds a user by email, ignoring case.
func ByEmail(ctx context.Context, db *gorm.DB, email string) (User, error) {
	var u User
	err := db.WithContext(ctx).Where("email = ?", NormalizeEmail(email)).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// ByID finds a user by ID.
func ByID(ctx context.Context, db *gorm.DB, id string) (User, error) {
	var u User
	err := db.WithContext(ctx).Where("id = ?", id).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// SetRole sets the role of a user who has none yet. A role is picked once.
func SetRole(ctx context.Context, db *gorm.DB, id string, role Role) error {
	res := db.WithContext(ctx).Model(&User{}).
		Where("id = ? AND role IS NULL", id).
		Update("role", role)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRoleSet
	}
	return nil
}

// SetFirstName saves the user's first name.
func SetFirstName(ctx context.Context, db *gorm.DB, id, firstName string) error {
	return db.WithContext(ctx).Model(&User{}).Where("id = ?", id).
		Update("first_name", strings.TrimSpace(firstName)).Error
}

// SetTimeZone saves the browser's time zone. An unknown one is ignored, so
// a bad value can never stop a sign-in.
func SetTimeZone(ctx context.Context, db *gorm.DB, id, zone string) error {
	if zone == "" || len(zone) > 64 {
		return nil
	}
	if _, err := time.LoadLocation(zone); err != nil {
		return nil
	}
	return db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Update("time_zone", zone).Error
}

// NormalizeEmail trims and lowercases an email, the form it is stored in.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
