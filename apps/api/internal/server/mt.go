package server

import (
	"context"
	"errors"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/id"
	"github.com/portierglobal/hijau/apps/api/internal/mt"
)

type mtConfigReq struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"apiKey"`
	Enabled  *bool  `json:"enabled"`
}

type mtConfigDTO struct {
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	Enabled        bool   `json:"enabled"`
	HasCredentials bool   `json:"hasCredentials"`
}

func (s *Server) getMTConfig(ctx context.Context, path *extractor.Path[projectPath]) (espresso.JSON[mtConfigDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[mtConfigDTO]{}, err
	}
	row, err := s.store.GetMTConfig(ctx, pid)
	if err != nil {
		return espresso.JSON[mtConfigDTO]{Data: mtConfigDTO{Enabled: false}}, nil // not configured
	}
	return espresso.JSON[mtConfigDTO]{Data: mtConfigDTO{
		Provider: row.Provider, Model: row.Model, Enabled: row.Enabled, HasCredentials: len(row.Credentials) > 0,
	}}, nil
}

// configureMT sets the project's MT provider. The API key is sealed at rest; an
// omitted apiKey keeps the existing one (so you can flip enabled/model without
// resending the secret).
func (s *Server) configureMT(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[mtConfigReq]) (espresso.JSON[mtConfigDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectAdmin, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[mtConfigDTO]{}, err
	}
	d := body.Data
	if d.Provider == "" {
		return espresso.JSON[mtConfigDTO]{}, espresso.ErrBadRequest("provider is required")
	}
	// Validate the provider name (constructing with a dummy key is cheap).
	if _, err := mt.NewProvider(mt.Config{Provider: d.Provider, APIKey: "x"}); err != nil {
		return espresso.JSON[mtConfigDTO]{}, espresso.ErrBadRequest(err.Error())
	}

	var creds []byte
	switch {
	case d.APIKey != "":
		if s.cipher == nil {
			return espresso.JSON[mtConfigDTO]{}, espresso.ErrInternal("server has no HIJAU_ENCRYPTION_KEY; cannot store credentials")
		}
		sealed, err := s.cipher.Seal([]byte(d.APIKey))
		if err != nil {
			return espresso.JSON[mtConfigDTO]{}, espresso.ErrInternal("could not seal credentials")
		}
		creds = sealed
	default:
		if existing, err := s.store.GetMTConfig(ctx, pid); err == nil {
			creds = existing.Credentials // preserve
		}
	}
	if (d.Provider == "claude" || d.Provider == "anthropic") && len(creds) == 0 {
		return espresso.JSON[mtConfigDTO]{}, espresso.ErrBadRequest("the claude provider requires an apiKey")
	}

	enabled := true
	if d.Enabled != nil {
		enabled = *d.Enabled
	}
	row, err := s.store.UpsertMTConfig(ctx, db.UpsertMTConfigParams{
		ID: id.New(), ProjectID: pid, Provider: d.Provider, Enabled: enabled, Model: d.Model, Credentials: creds,
	})
	if err != nil {
		return espresso.JSON[mtConfigDTO]{}, espresso.ErrInternal("could not save MT config")
	}
	return espresso.JSON[mtConfigDTO]{Data: mtConfigDTO{
		Provider: row.Provider, Model: row.Model, Enabled: row.Enabled, HasCredentials: len(row.Credentials) > 0,
	}}, nil
}

type mtSuggestReq struct {
	TargetLang string `json:"targetLang"`
}

// suggestMT returns a machine translation of a key's base-language text into the
// target language, with the ICU placeholder guard applied.
func (s *Server) suggestMT(ctx context.Context, path *extractor.Path[keyPath], body *espresso.JSON[mtSuggestReq]) (espresso.JSON[mt.Result], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermMTUse, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[mt.Result]{}, err
	}
	if body.Data.TargetLang == "" {
		return espresso.JSON[mt.Result]{}, espresso.ErrBadRequest("targetLang is required")
	}
	cfg, err := s.store.GetMTConfig(ctx, pid)
	if err != nil || !cfg.Enabled {
		return espresso.JSON[mt.Result]{}, espresso.ErrBadRequest("machine translation is not configured for this project")
	}
	key, err := s.store.GetKey(ctx, path.Data.KID)
	if err != nil || key.ProjectID != pid {
		return espresso.JSON[mt.Result]{}, espresso.ErrNotFound("key not found")
	}
	proj, err := s.store.GetProject(ctx, pid)
	if err != nil || proj.BaseLanguageID.String == "" {
		return espresso.JSON[mt.Result]{}, espresso.ErrPreconditionFailed("the project has no base language")
	}
	baseLang, err := s.store.GetLanguage(ctx, proj.BaseLanguageID.String)
	if err != nil {
		return espresso.JSON[mt.Result]{}, espresso.ErrInternal("base language lookup failed")
	}

	var source string
	trs, err := s.store.ListTranslationsForKey(ctx, key.ID)
	if err != nil {
		return espresso.JSON[mt.Result]{}, espresso.ErrInternal("could not load source text")
	}
	for _, t := range trs {
		if t.LanguageID == proj.BaseLanguageID.String {
			source = t.Text.String
		}
	}
	if source == "" {
		return espresso.JSON[mt.Result]{}, espresso.ErrPreconditionFailed("the base-language text is empty; nothing to translate")
	}

	apiKey, err := s.openMTKey(cfg)
	if err != nil {
		return espresso.JSON[mt.Result]{}, espresso.ErrInternal("could not read MT credentials")
	}
	provider, err := mt.NewProvider(mt.Config{Provider: cfg.Provider, APIKey: apiKey, Model: cfg.Model})
	if err != nil {
		return espresso.JSON[mt.Result]{}, espresso.ErrBadRequest(err.Error())
	}

	targetLangID := ""
	if tl, err := s.store.GetLanguageByTag(ctx, db.GetLanguageByTagParams{ProjectID: pid, Tag: body.Data.TargetLang}); err == nil {
		targetLangID = tl.ID
	}
	res, err := mt.GuardedTranslate(ctx, provider, mt.Request{
		Source: source, SourceLang: baseLang.Tag, TargetLang: body.Data.TargetLang,
		KeyName: key.Name, Description: key.Description.String,
		Glossary: s.buildGlossaryHints(ctx, pid, targetLangID, source),
	})
	if err != nil {
		var mismatch *mt.ErrPlaceholderMismatch
		if errors.As(err, &mismatch) {
			return espresso.JSON[mt.Result]{}, espresso.ErrBadRequest("the machine translation changed ICU placeholders; not applied")
		}
		// Don't reflect the raw upstream provider error (HTTP status, request IDs,
		// response-body fragments) to the client.
		return espresso.JSON[mt.Result]{}, espresso.ErrServiceUnavailable("machine translation is temporarily unavailable")
	}
	return espresso.JSON[mt.Result]{Data: res}, nil
}

func (s *Server) openMTKey(cfg db.MtConfig) (string, error) {
	if len(cfg.Credentials) == 0 {
		return "", nil // keyless provider (e.g. mock)
	}
	if s.cipher == nil {
		return "", errors.New("no encryption key configured")
	}
	plain, err := s.cipher.Open(cfg.Credentials)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
