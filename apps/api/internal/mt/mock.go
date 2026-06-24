package mt

import (
	"context"
	"strings"
)

// Mock is a keyless provider for tests and offline demos. It echoes the source
// (so ICU placeholders are trivially preserved) tagged with the target language,
// letting the whole suggest/auto-translate pipeline run without an API key. It
// reflects any glossary hints it received in Notes so the wiring is observable.
type Mock struct{}

func (Mock) Name() string { return "mock" }

func (Mock) Translate(_ context.Context, req Request) (Result, error) {
	r := Result{
		Text:     "[" + req.TargetLang + "] " + req.Source,
		Provider: "mock",
		Model:    "echo",
	}
	if len(req.Glossary) > 0 {
		parts := make([]string, len(req.Glossary))
		for i, g := range req.Glossary {
			if g.DoNotTranslate {
				parts[i] = g.Term + "=DNT"
			} else {
				parts[i] = g.Term + "=" + g.Translation
			}
		}
		r.Notes = "glossary: " + strings.Join(parts, ", ")
	}
	return r, nil
}
