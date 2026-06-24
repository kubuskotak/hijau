package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// TokenKind is the human-facing segment embedded in a generated token. It maps
// loosely to the api_key_type / session distinction.
type TokenKind string

const (
	KindSession TokenKind = "sess"
	KindPAT     TokenKind = "pat"
	KindProject TokenKind = "proj"
	KindEditor  TokenKind = "edit"
)

// GeneratedToken is produced when minting a token. Raw is shown to the caller
// exactly once; only Hash is persisted, and Prefix is stored for display in the
// UI so a user can identify a token without revealing it.
type GeneratedToken struct {
	Raw    string
	Hash   string
	Prefix string
}

// GenerateToken mints a random token of the form `hj_<kind>_<random>`.
func GenerateToken(kind TokenKind) (GeneratedToken, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return GeneratedToken{}, err
	}
	raw := fmt.Sprintf("hj_%s_%s", kind, base64.RawURLEncoding.EncodeToString(buf))
	prefix := raw
	if len(raw) > 14 {
		prefix = raw[:14]
	}
	return GeneratedToken{Raw: raw, Hash: HashToken(raw), Prefix: prefix}, nil
}

// HashToken returns the hex-encoded SHA-256 of a raw token. The hash is the
// unique lookup key stored in the database; raw tokens are never persisted.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// TokenKindFromRaw extracts the kind from a raw `hj_<kind>_...` token.
func TokenKindFromRaw(raw string) (TokenKind, bool) {
	parts := strings.SplitN(raw, "_", 3)
	if len(parts) != 3 || parts[0] != "hj" {
		return "", false
	}
	return TokenKind(parts[1]), true
}
