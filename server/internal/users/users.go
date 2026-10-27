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
	Role      Role `gorm:"default:null"`
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
	u := User{Email: normalizeEmail(email), Role: role}
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
	err := db.WithContext(ctx).Where("email = ?", normalizeEmail(email)).First(&u).Error
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

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
