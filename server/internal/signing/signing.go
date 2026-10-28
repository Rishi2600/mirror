// Package signing makes links that work only until they expire, such as a
// CV download (15 minutes, per the spec).
package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"time"
)

var (
	ErrExpired = errors.New("this link has expired; open the match again for a new one")
	ErrInvalid = errors.New("this link is not valid")
)

// Sign returns path with an expiry time and a signature added as query
// parameters.
func Sign(key []byte, path string, expires time.Time) string {
	exp := strconv.FormatInt(expires.Unix(), 10)
	q := url.Values{"exp": {exp}, "sig": {mac(key, path, exp)}}
	return path + "?" + q.Encode()
}

// Verify checks a link made by Sign for path, using its exp and sig values.
func Verify(key []byte, path, exp, sig string, now time.Time) error {
	if !hmac.Equal([]byte(sig), []byte(mac(key, path, exp))) {
		return ErrInvalid
	}
	unix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return ErrInvalid
	}
	if now.After(time.Unix(unix, 0)) {
		return ErrExpired
	}
	return nil
}

// mac signs the path and expiry together, so neither can be changed.
func mac(key []byte, path, exp string) string {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(path + "\n" + exp))
	return hex.EncodeToString(h.Sum(nil))
}
