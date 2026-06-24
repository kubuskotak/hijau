package mt

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// defaultClaudeModel is the cheap, vision-capable bulk model. Pass
// claude-sonnet-4-6 for review-grade translations.
const defaultClaudeModel = "claude-haiku-4-5"

// Claude translates via the Anthropic Messages API. The ICU placeholder
// contract is enforced by GuardedTranslate; the prompt also states it so the
// model gets it right the first time.
type Claude struct {
	apiKey string
	model  string
}

func NewClaude(apiKey, model string) *Claude {
	if model == "" {
		model = defaultClaudeModel
	}
	return &Claude{apiKey: apiKey, model: model}
}

func (c *Claude) Name() string { return "claude" }

func (c *Claude) Translate(ctx context.Context, req Request) (Result, error) {
	if c.apiKey == "" {
		return Result{}, fmt.Errorf("claude: no API key configured")
	}
	client := anthropic.NewClient(option.WithAPIKey(c.apiKey))
	msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(buildPrompt(req))),
		},
	})
	if err != nil {
		return Result{}, fmt.Errorf("claude: %w", err)
	}
	var sb strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return Result{}, fmt.Errorf("claude: empty response")
	}
	return Result{Text: text, Provider: "claude", Model: c.model}, nil
}

func buildPrompt(req Request) string {
	var b strings.Builder
	b.WriteString("You are a professional software-localization translator.\n")
	fmt.Fprintf(&b, "Translate the UI text from %s to %s.\n\n", sourceLangOrAuto(req.SourceLang), req.TargetLang)
	b.WriteString("Rules:\n")
	b.WriteString("- Output ONLY the translation — no quotes, labels, or explanation.\n")
	b.WriteString("- Preserve ICU placeholders EXACTLY (e.g. {name}, {count}); never translate or reorder their contents.\n")
	if len(req.Placeholders) > 0 {
		fmt.Fprintf(&b, "- The translation MUST contain these placeholders verbatim: %s\n", strings.Join(withBraces(req.Placeholders), ", "))
	}
	b.WriteString("- Keep the tone and length appropriate for interface copy.\n")
	if req.KeyName != "" {
		fmt.Fprintf(&b, "\nKey (context only): %s\n", req.KeyName)
	}
	if req.Description != "" {
		fmt.Fprintf(&b, "Description (context only): %s\n", req.Description)
	}
	for _, g := range req.Glossary {
		switch {
		case g.DoNotTranslate:
			fmt.Fprintf(&b, "Glossary: keep the term %q untranslated.\n", g.Term)
		case g.Translation != "":
			fmt.Fprintf(&b, "Glossary: translate %q as %q.\n", g.Term, g.Translation)
		}
	}
	if req.RepairNote != "" {
		fmt.Fprintf(&b, "\nIMPORTANT: %s\n", req.RepairNote)
	}
	b.WriteString("\nText to translate:\n")
	b.WriteString(req.Source)
	return b.String()
}

func sourceLangOrAuto(s string) string {
	if s == "" {
		return "the source language"
	}
	return s
}

func withBraces(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "{" + n + "}"
	}
	return out
}
