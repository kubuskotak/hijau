package crypto

import "testing"

func TestSealOpenRoundTrip(t *testing.T) {
	c, err := New("a-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("sk-ant-secret-key-123")
	sealed, err := c.Seal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if string(sealed) == string(plain) {
		t.Fatal("sealed output equals plaintext")
	}
	got, err := c.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plain) {
		t.Fatalf("round trip mismatch: got %q", got)
	}
}

func TestSealIsNonDeterministic(t *testing.T) {
	c, _ := New("k")
	a, _ := c.Seal([]byte("same"))
	b, _ := c.Seal([]byte("same"))
	if string(a) == string(b) {
		t.Fatal("expected distinct nonces to produce distinct ciphertexts")
	}
}

func TestOpenWithWrongKeyFails(t *testing.T) {
	c1, _ := New("key-one")
	c2, _ := New("key-two")
	sealed, _ := c1.Seal([]byte("secret"))
	if _, err := c2.Open(sealed); err == nil {
		t.Fatal("expected open under a different key to fail")
	}
}

func TestTamperFails(t *testing.T) {
	c, _ := New("k")
	sealed, _ := c.Seal([]byte("secret"))
	sealed[len(sealed)-1] ^= 0xff
	if _, err := c.Open(sealed); err == nil {
		t.Fatal("expected tampered ciphertext to fail authentication")
	}
}

func TestNoKey(t *testing.T) {
	if _, err := New(""); err != ErrNoKey {
		t.Fatalf("expected ErrNoKey, got %v", err)
	}
}
