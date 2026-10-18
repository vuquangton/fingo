package auth_test

import (
	"strings"
	"testing"

	"fingo/internal/auth"
)

func TestHashAndVerify(t *testing.T) {
	const password = "mật-khẩu-2026"

	encoded, err := auth.Hash(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Fatalf("hash is not PHC argon2id: %q", encoded)
	}
	if strings.Contains(encoded, password) {
		t.Fatal("the hash contains the password")
	}

	if err := auth.Verify(encoded, password); err != nil {
		t.Fatalf("verify the right password: %v", err)
	}
	if err := auth.Verify(encoded, password+"x"); err == nil {
		t.Fatal("a wrong password verified")
	}
}

func TestHashIsSaltedPerCall(t *testing.T) {
	a, err := auth.Hash("same-password")
	if err != nil {
		t.Fatalf("hash a: %v", err)
	}
	b, err := auth.Hash("same-password")
	if err != nil {
		t.Fatalf("hash b: %v", err)
	}

	if a == b {
		t.Fatal("two hashes of one password are identical: the salt is not random")
	}

	if err := auth.Verify(a, "same-password"); err != nil {
		t.Fatalf("verify a: %v", err)
	}
	if err := auth.Verify(b, "same-password"); err != nil {
		t.Fatalf("verify b: %v", err)
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
	}{
		{"empty", ""},
		{"not phc", "plaintext"},
		{"wrong algorithm", "$argon2i$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA"},
		{"wrong version", "$argon2id$v=16$m=65536,t=3,p=2$c2FsdA$aGFzaA"},
		{"bad parameters", "$argon2id$v=19$m=zero,t=3,p=2$c2FsdA$aGFzaA"},
		{"zero memory", "$argon2id$v=19$m=0,t=3,p=2$c2FsdA$aGFzaA"},
		{"bad base64", "$argon2id$v=19$m=65536,t=3,p=2$!!!$!!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := auth.Verify(tt.encoded, "anything"); err == nil {
				t.Fatalf("Verify(%q) accepted a malformed hash", tt.encoded)
			}
		})
	}
}
