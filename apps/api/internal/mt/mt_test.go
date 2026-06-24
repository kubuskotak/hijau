package mt

import (
	"context"
	"errors"
	"testing"
)

func TestGuardedTranslateMockPreservesPlaceholders(t *testing.T) {
	res, err := GuardedTranslate(context.Background(), Mock{}, Request{
		Source: "Hello {name}, you have {count} messages", TargetLang: "fr",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Provider != "mock" {
		t.Fatalf("provider = %q", res.Provider)
	}
	// Mock echoes the source, so both placeholders survive.
	if want := "[fr] Hello {name}, you have {count} messages"; res.Text != want {
		t.Fatalf("text = %q, want %q", res.Text, want)
	}
}

// dropper returns a translation missing a placeholder, every time.
type dropper struct{}

func (dropper) Name() string { return "dropper" }
func (dropper) Translate(_ context.Context, req Request) (Result, error) {
	return Result{Text: "Bonjour", Provider: "dropper"}, nil
}

func TestGuardedTranslateRejectsDroppedPlaceholders(t *testing.T) {
	_, err := GuardedTranslate(context.Background(), dropper{}, Request{
		Source: "Hello {name}", TargetLang: "fr",
	})
	var mismatch *ErrPlaceholderMismatch
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected ErrPlaceholderMismatch, got %v", err)
	}
	if len(mismatch.Want) != 1 || mismatch.Want[0] != "name" {
		t.Fatalf("want placeholders = %v", mismatch.Want)
	}
}

func TestGuardedTranslateNoPlaceholders(t *testing.T) {
	res, err := GuardedTranslate(context.Background(), Mock{}, Request{Source: "Save", TargetLang: "de"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "[de] Save" {
		t.Fatalf("text = %q", res.Text)
	}
}

func TestNewProvider(t *testing.T) {
	if _, err := NewProvider(Config{Provider: "mock"}); err != nil {
		t.Fatalf("mock: %v", err)
	}
	if _, err := NewProvider(Config{Provider: "claude", APIKey: "k"}); err != nil {
		t.Fatalf("claude: %v", err)
	}
	if _, err := NewProvider(Config{Provider: "nope"}); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
