package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"gorm.io/gorm"

	"mirror/server/internal/users"
)

// SessionTTL is how long a session lasts after sign-in.
const SessionTTL = 30 * 24 * time.Hour

var ErrNoSession = errors.New("not signed in")

type session struct {
	ID        string `gorm:"type:uuid;default:gen_random_uuid()"`
	TokenHash string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// CreateSession starts a session for a user and returns its token. Only
// the token's hash is stored.
func CreateSession(ctx context.Context, db *gorm.DB, userID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)

	s := session{TokenHash: hash(token), UserID: userID, ExpiresAt: time.Now().Add(SessionTTL)}
	if err := db.WithContext(ctx).Create(&s).Error; err != nil {
		return "", err
	}
	return token, nil
}

// UserForSession returns the signed-in user for a session token.
func UserForSession(ctx context.Context, db *gorm.DB, token string) (users.User, error) {
	if token == "" {
		return users.User{}, ErrNoSession
	}
	var s session
	err := db.WithContext(ctx).
		Where("token_hash = ? AND expires_at > ?", hash(token), time.Now()).
		First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return users.User{}, ErrNoSession
	}
	if err != nil {
		return users.User{}, err
	}
	return users.ByID(ctx, db, s.UserID)
}

// DeleteSession ends a session. An unknown token is not an error.
func DeleteSession(ctx context.Context, db *gorm.DB, token string) error {
	return db.WithContext(ctx).Where("token_hash = ?", hash(token)).Delete(&session{}).Error
}
