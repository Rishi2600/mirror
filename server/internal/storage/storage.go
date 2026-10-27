// Package storage keeps uploaded files on local disk, encrypted.
package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrBadKey = errors.New("invalid file key")

// Local stores each file as one AES-256-GCM encrypted file in dir:
// a random 12-byte nonce followed by the sealed bytes.
type Local struct {
	dir  string
	aead cipher.AEAD
}

// NewLocal creates dir if needed. secret must be 32 bytes.
func NewLocal(dir string, secret []byte) (*Local, error) {
	if len(secret) != 32 {
		return nil, fmt.Errorf("the file encryption key must be 32 bytes, got %d", len(secret))
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &Local{dir: dir, aead: aead}, nil
}

// Save encrypts and writes data, and returns the key to read it back.
func (s *Local) Save(data []byte) (string, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return "", err
	}
	key := hex.EncodeToString(id)

	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	// The key is bound in as extra data, so swapped files fail to decrypt.
	sealed := s.aead.Seal(nonce, nonce, data, []byte(key))

	// Write to a temp file first, so a crash never leaves half a file.
	tmp := filepath.Join(s.dir, key+".tmp")
	if err := os.WriteFile(tmp, sealed, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, filepath.Join(s.dir, key)); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return key, nil
}

// Open reads and decrypts the file saved under key.
func (s *Local) Open(key string) ([]byte, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	sealed, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	n := s.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("stored file is too short")
	}
	return s.aead.Open(nil, sealed[:n], sealed[n:], []byte(key))
}

// Delete removes the file saved under key.
func (s *Local) Delete(key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// path checks that key is one of ours (32 hex characters), so it can never
// point outside dir.
func (s *Local) path(key string) (string, error) {
	if b, err := hex.DecodeString(key); err != nil || len(b) != 16 {
		return "", ErrBadKey
	}
	return filepath.Join(s.dir, key), nil
}
