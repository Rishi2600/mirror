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
	ID        string `gorm:"type:uuid;default:gen_random_uuid()"`
	Email     string
	Role      Role
	CreatedAt time.Time
}

var (
	ErrEmailTaken = errors.New("email is already registered")
	ErrNotFound   = errors.New("user not found")
)

// Create adds a user. The email is stored trimmed and lowercase.
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

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
