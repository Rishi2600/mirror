// Package auth handles sign-in codes and sessions.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	netmail "net/mail"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"mirror/server/internal/mail"
	"mirror/server/internal/users"
)

const (
	codeTTL        = 10 * time.Minute
	resendCooldown = 60 * time.Second
	maxAttempts    = 5
)

var (
	ErrInvalidEmail = errors.New("enter a valid email address")
	ErrWrongCode    = errors.New("that code is not right")
	ErrCodeExpired  = errors.New("that code has expired; request a new one")
	ErrTooManyTries = errors.New("too many wrong codes; request a new one")
)

// TooSoonError means a new code was requested before the cooldown ended.
type TooSoonError struct {
	RetryAfter time.Duration
}

func (e TooSoonError) Error() string {
	return fmt.Sprintf("wait %d seconds before asking for a new code", int(e.RetryAfter.Seconds()))
}

type otpCode struct {
	Email     string `gorm:"primaryKey"`
	CodeHash  string
	Attempts  int
	ExpiresAt time.Time
	CreatedAt time.Time
}

// RequestCode emails a new 6-digit sign-in code, replacing any earlier one.
func RequestCode(ctx context.Context, db *gorm.DB, mailer mail.Mailer, email string) error {
	email = users.NormalizeEmail(email)
	if addr, err := netmail.ParseAddress(email); err != nil || addr.Address != email {
		return ErrInvalidEmail
	}
	db = db.WithContext(ctx)

	var existing otpCode
	err := db.Where("email = ?", email).First(&existing).Error
	if err == nil {
		if wait := resendCooldown - time.Since(existing.CreatedAt); wait > 0 {
			return TooSoonError{RetryAfter: wait.Round(time.Second)}
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	code, err := newCode()
	if err != nil {
		return err
	}
	row := otpCode{
		Email:     email,
		CodeHash:  hash(code),
		ExpiresAt: time.Now().Add(codeTTL),
		CreatedAt: time.Now(),
	}
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
		return err
	}

	body := fmt.Sprintf("Your Mirror sign-in code is %s.\n\nIt expires in 10 minutes. If you did not ask for it, you can ignore this email.\n", code)
	if err := mailer.Send(ctx, email, "Your Mirror sign-in code", body); err != nil {
		// Remove the unsent code, so the user can ask again at once.
		db.Delete(&otpCode{}, "email = ?", email)
		return fmt.Errorf("sending the sign-in code: %w", err)
	}
	return nil
}

// VerifyCode checks a sign-in code and returns the user, creating the
// account on first sign-in. A code works once; five wrong tries remove it.
func VerifyCode(ctx context.Context, db *gorm.DB, email, code string) (users.User, error) {
	email = users.NormalizeEmail(email)
	var user users.User
	var codeErr error

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row otpCode
		// Lock the row, so two checks of the same code cannot both pass.
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("email = ?", email).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			codeErr = ErrWrongCode
			return nil
		}
		if err != nil {
			return err
		}

		if time.Now().After(row.ExpiresAt) {
			codeErr = ErrCodeExpired
			return tx.Delete(&row).Error
		}
		if subtle.ConstantTimeCompare([]byte(hash(strings.TrimSpace(code))), []byte(row.CodeHash)) != 1 {
			row.Attempts++
			if row.Attempts >= maxAttempts {
				codeErr = ErrTooManyTries
				return tx.Delete(&row).Error
			}
			codeErr = ErrWrongCode
			return tx.Model(&row).Update("attempts", row.Attempts).Error
		}

		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		user, err = findOrCreateUser(ctx, tx, email)
		return err
	})
	if err != nil {
		return users.User{}, err
	}
	if codeErr != nil {
		return users.User{}, codeErr
	}
	return user, nil
}

func findOrCreateUser(ctx context.Context, db *gorm.DB, email string) (users.User, error) {
	u, err := users.ByEmail(ctx, db, email)
	if errors.Is(err, users.ErrNotFound) {
		return users.Create(ctx, db, email, "")
	}
	return u, err
}

func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// hash returns the SHA-256 of s in hex. Codes and tokens are stored only as
// hashes, so a database leak does not reveal live ones.
func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
