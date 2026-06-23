package mt

import "context"

// Mock is a keyless provider for tests and offline demos. It echoes the source
// (so ICU placeholders are trivially preserved) tagged with the target language,
// letting the whole suggest/auto-translate pipeline run without an API key.
type Mock struct{}

func (Mock) Name() string { return "mock" }

func (Mock) Translate(_ context.Context, req Request) (Result, error) {
	return Result{
		Text:     "[" + req.TargetLang + "] " + req.Source,
		Provider: "mock",
		Model:    "echo",
	}, nil
}
