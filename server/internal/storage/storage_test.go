package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

var testSecret = bytes.Repeat([]byte{7}, 32)

func TestSaveOpenDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocal(dir, testSecret)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("Jordan Example, backend engineer")

	key, err := s.Save(data)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	onDisk, _ := os.ReadFile(filepath.Join(dir, key))
	if bytes.Contains(onDisk, []byte("Jordan")) {
		t.Error("the file on disk is not encrypted")
	}

	got, err := s.Open(key)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("Open = %q, %v; want %q", got, err, data)
	}

	if err := s.Delete(key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Open(key); err == nil {
		t.Error("Open succeeded after Delete")
	}
}

func TestWrongSecretCannotRead(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewLocal(dir, testSecret)
	key, _ := s.Save([]byte("secret cv"))

	other, _ := NewLocal(dir, bytes.Repeat([]byte{8}, 32))
	if _, err := other.Open(key); err == nil {
		t.Error("a different secret decrypted the file")
	}
}

func TestBadKeysAreRejected(t *testing.T) {
	s, _ := NewLocal(t.TempDir(), testSecret)
	for _, key := range []string{"", "../../etc/passwd", "abc", "zz112233445566778899aabbccddeeff"} {
		if _, err := s.Open(key); !errors.Is(err, ErrBadKey) {
			t.Errorf("Open(%q) error = %v, want ErrBadKey", key, err)
		}
	}
}

func TestSecretMustBe32Bytes(t *testing.T) {
	if _, err := NewLocal(t.TempDir(), []byte("short")); err == nil {
		t.Error("NewLocal accepted a 5-byte secret")
	}
}
