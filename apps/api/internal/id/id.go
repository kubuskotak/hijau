// Package id generates lexicographically-sortable unique identifiers (ULID).
//
// ULIDs sort by creation time, which makes them convenient as primary keys and
// for cursor pagination. Entropy comes from crypto/rand, which is safe for
// concurrent use (unlike ulid.Make's default monotonic source).
package id

import (
	"crypto/rand"

	"github.com/oklog/ulid/v2"
)

// New returns a new 26-character ULID string.
func New() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}
