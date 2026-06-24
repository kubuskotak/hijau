// Package i18n provides the server-side ICU/i18n logic shared by the translation
// write path, import/export, and machine translation: placeholder validation and
// source-change (OUTDATED) detection.
//
// This is a pragmatic M1 validator, not a full ICU parser. It validates simple
// {name} interpolation placeholders and brace balance; complex ICU constructs
// ({count, plural, ...}, {gender, select, ...}) are recognized for brace
// balance but their inner sub-messages are intentionally not treated as
// placeholders. The canonical cross-language behaviour is pinned by the shared
// fixture corpus that both this package and the TS @hijau/i18n package test
// against.
package i18n

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

func isIdent(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// SimplePlaceholders returns the sorted, unique simple `{name}` interpolation
// placeholders in an ICU message. Brace groups whose content is not a bare
// identifier (e.g. `{count, plural, ...}` or a plural sub-message `{# items}`)
// are ignored, so this does not false-positive on ICU format constructs.
func SimplePlaceholders(s string) []string {
	seen := map[string]bool{}
	var out []string
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '\'':
			// ICU apostrophe quoting: '' is a literal apostrophe; otherwise a
			// quoted span runs until the next apostrophe.
			if i+1 < len(rs) && rs[i+1] == '\'' {
				i++
				continue
			}
			i++
			for i < len(rs) && rs[i] != '\'' {
				i++
			}
		case '{':
			j := i + 1
			start := j
			for j < len(rs) && isIdent(rs[j]) {
				j++
			}
			if j > start && j < len(rs) && rs[j] == '}' {
				name := string(rs[start:j])
				if !seen[name] {
					seen[name] = true
					out = append(out, name)
				}
				i = j
			}
		}
	}
	sort.Strings(out)
	return out
}

// BracesBalanced reports whether `{`/`}` are balanced, ignoring quoted spans.
func BracesBalanced(s string) bool {
	depth := 0
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '\'':
			if i+1 < len(rs) && rs[i+1] == '\'' {
				i++
				continue
			}
			i++
			for i < len(rs) && rs[i] != '\'' {
				i++
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// isComplexICU reports whether a message contains an ICU format construct of
// the form `{arg, type, ...}` (plural, select, number, date, ...). For those,
// M1 only validates brace balance — the naive simple-placeholder scan would
// otherwise misread plural/select sub-message text as placeholders.
func isComplexICU(s string) bool {
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] != '{' {
			continue
		}
		j := i + 1
		for j < len(rs) && rs[j] == ' ' {
			j++
		}
		start := j
		for j < len(rs) && isIdent(rs[j]) {
			j++
		}
		if j == start {
			continue
		}
		for j < len(rs) && rs[j] == ' ' {
			j++
		}
		if j < len(rs) && rs[j] == ',' {
			return true
		}
	}
	return false
}

// ValidateTranslation checks that a target message is well-formed and does not
// introduce simple placeholders absent from the base (source) message. A target
// may omit base placeholders (some locales legitimately do), but must not invent
// new ones. Complex ICU messages are only brace-balance checked in M1.
func ValidateTranslation(target, base string) error {
	if strings.TrimSpace(target) == "" {
		return nil // empty target = untranslated; nothing to validate
	}
	if !BracesBalanced(target) {
		return fmt.Errorf("unbalanced braces in translation")
	}
	if isComplexICU(target) || isComplexICU(base) {
		return nil
	}
	baseSet := map[string]bool{}
	for _, p := range SimplePlaceholders(base) {
		baseSet[p] = true
	}
	var unknown []string
	for _, p := range SimplePlaceholders(target) {
		if !baseSet[p] {
			unknown = append(unknown, p)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("translation uses unknown placeholder(s): %s", strings.Join(unknown, ", "))
	}
	return nil
}

// Normalize trims surrounding whitespace; used so trivial whitespace edits to a
// source string don't mark every translation OUTDATED.
func Normalize(s string) string { return strings.TrimSpace(s) }

// SourceHash returns a stable hash of the normalized source text.
func SourceHash(s string) string {
	sum := sha256.Sum256([]byte(Normalize(s)))
	return hex.EncodeToString(sum[:])
}
