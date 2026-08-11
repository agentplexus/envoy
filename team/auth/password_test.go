package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := hashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Errorf("hash not in argon2id PHC form: %q", hash)
	}
	if strings.Contains(hash, "correct horse battery") {
		t.Error("plaintext leaked into the hash")
	}
	if !verifyPassword(hash, "correct horse battery") {
		t.Error("verify of the correct password failed")
	}
	if verifyPassword(hash, "wrong password") {
		t.Error("verify of a wrong password succeeded")
	}
}

func TestHashPassword_TooShort(t *testing.T) {
	if _, err := hashPassword("short"); err != ErrWeakPassword {
		t.Errorf("err = %v, want ErrWeakPassword", err)
	}
}

func TestHashPassword_DistinctSalts(t *testing.T) {
	h1, _ := hashPassword("same-password-here")
	h2, _ := hashPassword("same-password-here")
	if h1 == h2 {
		t.Error("two hashes of the same password are identical — salt not random")
	}
	if !verifyPassword(h1, "same-password-here") || !verifyPassword(h2, "same-password-here") {
		t.Error("both hashes should verify the same password")
	}
}

func TestVerifyPassword_Malformed(t *testing.T) {
	for _, enc := range []string{"", "notahash", "$argon2id$bad", "$bcrypt$v=19$m=1,t=1,p=1$x$y"} {
		if verifyPassword(enc, "whatever") {
			t.Errorf("malformed encoding %q verified as valid", enc)
		}
	}
}
