package signing

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

var key = []byte("0123456789abcdef0123456789abcdef")

func parts(t *testing.T, link string) (path, exp, sig string) {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	return u.Path, u.Query().Get("exp"), u.Query().Get("sig")
}

func TestSignAndVerify(t *testing.T) {
	now := time.Now()
	path, exp, sig := parts(t, Sign(key, "/v1/matches/abc/cv", now.Add(15*time.Minute)))

	if err := Verify(key, path, exp, sig, now); err != nil {
		t.Errorf("a fresh link: %v", err)
	}
	if err := Verify(key, path, exp, sig, now.Add(16*time.Minute)); !errors.Is(err, ErrExpired) {
		t.Errorf("after 16 minutes: error = %v, want ErrExpired", err)
	}
	if err := Verify(key, "/v1/matches/other/cv", exp, sig, now); !errors.Is(err, ErrInvalid) {
		t.Errorf("another path: error = %v, want ErrInvalid", err)
	}
	later := strings.Replace(exp, exp[:1], "9", 1)
	if err := Verify(key, path, later, sig, now); !errors.Is(err, ErrInvalid) {
		t.Errorf("a changed expiry: error = %v, want ErrInvalid", err)
	}
	if err := Verify([]byte("another key, 32 bytes long......"), path, exp, sig, now); !errors.Is(err, ErrInvalid) {
		t.Errorf("another key: error = %v, want ErrInvalid", err)
	}
}
