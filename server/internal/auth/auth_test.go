package auth

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"gorm.io/gorm"

	"mirror/server/internal/dbtest"
	"mirror/server/internal/users"
)

// fakeMailer records the last email instead of sending it.
type fakeMailer struct {
	to, body string
	err      error
}

func (m *fakeMailer) Send(ctx context.Context, to, subject, body string) error {
	m.to, m.body = to, body
	return m.err
}

var sixDigits = regexp.MustCompile(`\b\d{6}\b`)

// requestCode asks for a code and returns the one that was emailed.
func requestCode(t *testing.T, tx *gorm.DB, email string) string {
	t.Helper()
	m := &fakeMailer{}
	if err := RequestCode(context.Background(), tx, m, email); err != nil {
		t.Fatalf("RequestCode: %v", err)
	}
	code := sixDigits.FindString(m.body)
	if code == "" {
		t.Fatalf("no 6-digit code in email body %q", m.body)
	}
	return code
}

func TestSignInCreatesUserOnce(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.Tx(t)

	code := requestCode(t, tx, "Grace.Test@Example.com")
	u, err := VerifyCode(ctx, tx, "grace.test@example.com", code)
	if err != nil {
		t.Fatalf("VerifyCode: %v", err)
	}
	if u.Email != "grace.test@example.com" || u.Role != "" {
		t.Errorf("new user = %+v, want lowercase email and no role", u)
	}

	// The code works once.
	if _, err := VerifyCode(ctx, tx, "grace.test@example.com", code); !errors.Is(err, ErrWrongCode) {
		t.Errorf("reusing a code: error = %v, want ErrWrongCode", err)
	}

	// Signing in again returns the same account. The cooldown is moved back
	// so a second code can be requested at once.
	tx.Exec("UPDATE otp_codes SET created_at = now() - interval '2 minutes'")
	code = requestCode(t, tx, "grace.test@example.com")
	tx.Exec("UPDATE otp_codes SET created_at = now() - interval '2 minutes'")
	again, err := VerifyCode(ctx, tx, "grace.test@example.com", code)
	if err != nil || again.ID != u.ID {
		t.Errorf("second sign-in = %+v, %v; want user %s", again, err, u.ID)
	}
}

func TestInvalidEmailIsRejected(t *testing.T) {
	tx := dbtest.Tx(t)
	for _, email := range []string{"", "not-an-email", "Name <a@example.com>"} {
		if err := RequestCode(context.Background(), tx, &fakeMailer{}, email); !errors.Is(err, ErrInvalidEmail) {
			t.Errorf("%q: error = %v, want ErrInvalidEmail", email, err)
		}
	}
}

func TestNewCodeMustWaitForCooldown(t *testing.T) {
	tx := dbtest.Tx(t)
	requestCode(t, tx, "wait@example.com")

	err := RequestCode(context.Background(), tx, &fakeMailer{}, "wait@example.com")
	var tooSoon TooSoonError
	if !errors.As(err, &tooSoon) || tooSoon.RetryAfter <= 0 {
		t.Errorf("error = %v, want TooSoonError with a wait", err)
	}
}

func TestFiveWrongCodesRemoveTheCode(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.Tx(t)
	code := requestCode(t, tx, "tries@example.com")
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}

	for i := 1; i <= 4; i++ {
		if _, err := VerifyCode(ctx, tx, "tries@example.com", wrong); !errors.Is(err, ErrWrongCode) {
			t.Fatalf("wrong try %d: error = %v, want ErrWrongCode", i, err)
		}
	}
	if _, err := VerifyCode(ctx, tx, "tries@example.com", wrong); !errors.Is(err, ErrTooManyTries) {
		t.Fatalf("wrong try 5: error = %v, want ErrTooManyTries", err)
	}
	if _, err := VerifyCode(ctx, tx, "tries@example.com", code); !errors.Is(err, ErrWrongCode) {
		t.Errorf("right code after 5 wrong tries: error = %v, want ErrWrongCode", err)
	}
}

func TestExpiredCodeIsRejected(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.Tx(t)
	code := requestCode(t, tx, "late@example.com")
	tx.Exec("UPDATE otp_codes SET expires_at = now() - interval '1 second'")

	if _, err := VerifyCode(ctx, tx, "late@example.com", code); !errors.Is(err, ErrCodeExpired) {
		t.Errorf("error = %v, want ErrCodeExpired", err)
	}
}

func TestFailedEmailAllowsAnotherRequest(t *testing.T) {
	tx := dbtest.Tx(t)
	err := RequestCode(context.Background(), tx, &fakeMailer{err: errors.New("smtp down")}, "retry@example.com")
	if err == nil {
		t.Fatal("RequestCode succeeded although the email failed")
	}
	requestCode(t, tx, "retry@example.com") // no cooldown after a failed send
}

func TestSessions(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.Tx(t)
	u, err := users.Create(ctx, tx, "session@example.com", users.RoleCandidate)
	if err != nil {
		t.Fatal(err)
	}

	token, err := CreateSession(ctx, tx, u.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	got, err := UserForSession(ctx, tx, token)
	if err != nil || got.ID != u.ID {
		t.Fatalf("UserForSession = %+v, %v; want user %s", got, err, u.ID)
	}

	// The database stores only the hash, never the token.
	var stored int64
	tx.Raw("SELECT count(*) FROM sessions WHERE token_hash = ?", token).Scan(&stored)
	if stored != 0 {
		t.Error("the raw token is stored in the database")
	}

	if _, err := UserForSession(ctx, tx, "made-up-token"); !errors.Is(err, ErrNoSession) {
		t.Errorf("unknown token: error = %v, want ErrNoSession", err)
	}

	if err := DeleteSession(ctx, tx, token); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := UserForSession(ctx, tx, token); !errors.Is(err, ErrNoSession) {
		t.Errorf("after sign-out: error = %v, want ErrNoSession", err)
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.Tx(t)
	u, _ := users.Create(ctx, tx, "old@example.com", users.RoleEmployer)
	token, _ := CreateSession(ctx, tx, u.ID)
	tx.Exec("UPDATE sessions SET expires_at = now() - interval '1 second'")

	if _, err := UserForSession(ctx, tx, token); !errors.Is(err, ErrNoSession) {
		t.Errorf("error = %v, want ErrNoSession", err)
	}
}
