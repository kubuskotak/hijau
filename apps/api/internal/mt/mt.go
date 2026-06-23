// Package mt provides machine-translation: a pluggable Provider interface, an
// ICU placeholder guard shared by all providers, and adapters (mock, Claude).
// MT stays disabled until a provider is configured for a project.
package mt

import (
	"context"
	"fmt"

	"github.com/portierglobal/hijau/apps/api/internal/i18n"
)

// GlossaryHint steers a translation toward approved terminology. An empty
// Translation with DoNotTranslate means "leave this term as-is".
type GlossaryHint struct {
	Term           string
	Translation    string
	DoNotTranslate bool
}

type Request struct {
	Source       string
	SourceLang   string // BCP-47
	TargetLang   string // BCP-47
	KeyName      string // optional context for the model
	Description  string // optional translator-facing context
	Placeholders []string // ICU placeholders to preserve (set by GuardedTranslate)
	Glossary     []GlossaryHint
}

type Result struct {
	Text     string `json:"text"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Notes    string `json:"notes,omitempty"`
}

// Provider is one machine-translation backend.
type Provider interface {
	Name() string
	Translate(ctx context.Context, req Request) (Result, error)
}

// ErrPlaceholderMismatch flags a translation that changed the ICU placeholder set.
type ErrPlaceholderMismatch struct{ Want, Got []string }

func (e *ErrPlaceholderMismatch) Error() string {
	return fmt.Sprintf("translation placeholders %v do not match source %v", e.Got, e.Want)
}

// GuardedTranslate runs a provider and enforces the ICU placeholder contract:
// the target must carry exactly the source's placeholder set. It records the
// expected placeholders on the request (so providers can instruct the model)
// and retries once before failing — protecting against an MT engine dropping or
// mangling `{name}`-style variables.
func GuardedTranslate(ctx context.Context, p Provider, req Request) (Result, error) {
	want := i18n.SimplePlaceholders(req.Source)
	req.Placeholders = want

	res, err := p.Translate(ctx, req)
	if err != nil {
		return res, err
	}
	if equalStrings(want, i18n.SimplePlaceholders(res.Text)) {
		return res, nil
	}
	// One repair attempt — the request already carries the required placeholders.
	res, err = p.Translate(ctx, req)
	if err != nil {
		return res, err
	}
	if got := i18n.SimplePlaceholders(res.Text); !equalStrings(want, got) {
		return res, &ErrPlaceholderMismatch{Want: want, Got: got}
	}
	return res, nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Config selects and configures a provider.
type Config struct {
	Provider string // "mock" | "claude"
	APIKey   string
	Model    string
}

// NewProvider constructs a provider from config.
func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case "mock":
		return Mock{}, nil
	case "claude", "anthropic":
		return NewClaude(cfg.APIKey, cfg.Model), nil
	default:
		return nil, fmt.Errorf("unknown MT provider %q", cfg.Provider)
	}
}
