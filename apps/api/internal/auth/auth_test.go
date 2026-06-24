package auth

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("s3cret-passphrase")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, err := VerifyPassword("s3cret-passphrase", h)
	if err != nil || !ok {
		t.Fatalf("expected match, ok=%v err=%v", ok, err)
	}
	bad, err := VerifyPassword("wrong-passphrase", h)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if bad {
		t.Fatal("expected mismatch for wrong password")
	}
}

func TestVerifyPasswordRejectsGarbage(t *testing.T) {
	if _, err := VerifyPassword("x", "not-a-hash"); err == nil {
		t.Fatal("expected error for malformed hash")
	}
}

func TestTokenMintAndHash(t *testing.T) {
	tok, err := GenerateToken(KindPAT)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if HashToken(tok.Raw) != tok.Hash {
		t.Fatal("hash is not stable for the raw token")
	}
	if tok.Prefix == "" || tok.Prefix == tok.Raw {
		t.Fatalf("unexpected prefix %q", tok.Prefix)
	}
	k, ok := TokenKindFromRaw(tok.Raw)
	if !ok || k != KindPAT {
		t.Fatalf("kind parse failed: kind=%q ok=%v", k, ok)
	}

	other, err := GenerateToken(KindPAT)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if other.Raw == tok.Raw {
		t.Fatal("two generated tokens collided")
	}
}
