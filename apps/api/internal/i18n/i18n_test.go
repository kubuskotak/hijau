package i18n

import (
	"reflect"
	"testing"
)

func TestSimplePlaceholders(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"Hello, world", nil},
		{"Hello {name}", []string{"name"}},
		{"Hi {name}, you have {count} items", []string{"count", "name"}},
		{"{name} and {name} again", []string{"name"}},
		// complex ICU: plural arg + sub-message text are NOT simple placeholders
		{"{count, plural, one {# item} other {# items}}", nil},
		// apostrophe-quoted brace is a literal, not a placeholder
		{"a '{' literal {x}", []string{"x"}},
	}
	for _, c := range cases {
		got := SimplePlaceholders(c.in)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("SimplePlaceholders(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestBracesBalanced(t *testing.T) {
	ok := []string{"", "no braces", "{a}", "{a, plural, one {x} other {y}}", "'{' quoted"}
	bad := []string{"{", "}", "{a} }", "{ {a} "}
	for _, s := range ok {
		if !BracesBalanced(s) {
			t.Errorf("BracesBalanced(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if BracesBalanced(s) {
			t.Errorf("BracesBalanced(%q) = true, want false", s)
		}
	}
}

func TestValidateTranslation(t *testing.T) {
	// subset / equal placeholders ok
	if err := ValidateTranslation("Hola {name}", "Hello {name}"); err != nil {
		t.Errorf("expected ok, got %v", err)
	}
	// omitting a base placeholder is allowed
	if err := ValidateTranslation("Hola", "Hello {name}"); err != nil {
		t.Errorf("expected ok (omitted placeholder), got %v", err)
	}
	// unknown placeholder rejected
	if err := ValidateTranslation("Hola {nombre}", "Hello {name}"); err == nil {
		t.Error("expected error for unknown placeholder")
	}
	// empty target ok
	if err := ValidateTranslation("", "Hello {name}"); err != nil {
		t.Errorf("expected ok for empty target, got %v", err)
	}
	// unbalanced braces rejected
	if err := ValidateTranslation("Hello {name", "Hello {name}"); err == nil {
		t.Error("expected error for unbalanced braces")
	}
	// complex ICU only brace-checked: differing sub-message text is allowed
	base := "{count, plural, one {# item} other {# items}}"
	target := "{count, plural, one {# artículo} other {# artículos}}"
	if err := ValidateTranslation(target, base); err != nil {
		t.Errorf("expected ok for complex ICU, got %v", err)
	}
}

func TestSourceHashStableUnderWhitespace(t *testing.T) {
	if SourceHash("Hello") != SourceHash("  Hello  ") {
		t.Error("source hash should ignore surrounding whitespace")
	}
	if SourceHash("Hello") == SourceHash("Hello!") {
		t.Error("source hash should differ for different content")
	}
}
