package main

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPasswordReadsOneLine(t *testing.T) {
	var output bytes.Buffer
	if err := hashPassword(strings.NewReader("correct horse battery staple\nignored\n"), &output); err != nil {
		t.Fatal(err)
	}
	hash := strings.TrimSpace(output.String())
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("correct horse battery staple")); err != nil {
		t.Fatalf("generated hash does not match password: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("ignored")) == nil {
		t.Fatal("hash-password read more than the first line")
	}
}

func TestHashPasswordRejectsEmptyPassword(t *testing.T) {
	if err := hashPassword(strings.NewReader("\n"), &bytes.Buffer{}); err == nil {
		t.Fatal("hashPassword() error = nil, want error")
	}
}
