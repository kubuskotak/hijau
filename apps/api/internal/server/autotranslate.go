package server

import (
	"context"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/i18n"
	"github.com/portierglobal/hijau/apps/api/internal/mt"
	"github.com/portierglobal/hijau/apps/api/internal/service"
)

type autoTranslateReq struct {
	TargetLang string `json:"targetLang"`
	Limit      int    `json:"limit"`
}

type autoTranslateDTO struct {
	TargetLang string `json:"targetLang"`
	Scanned    int    `json:"scanned"`
	Translated int    `json:"translated"`
	FromTM     int    `json:"fromTM"`
	FromMT     int    `json:"fromMT"`
	Skipped    int    `json:"skipped"`
	Failed     int    `json:"failed"`
}

// autoTranslate fills in untranslated keys for a target language: an exact
// translation-memory hit is reused (origin machine_tm); otherwise, if MT is
// configured, the base text is machine-translated with glossary hints through
// the ICU guard (origin machine_mt). Writes go through the normal transaction
// (history + activity + ICU validation), attributed to MT. Bounded per call.
func (s *Server) autoTranslate(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[autoTranslateReq]) (espresso.JSON[autoTranslateDTO], error) {
	pid := path.Data.PID
	if body.Data.TargetLang == "" {
		return espresso.JSON[autoTranslateDTO]{}, espresso.ErrBadRequest("targetLang is required")
	}
	targetLang, err := s.store.GetLanguageByTag(ctx, db.GetLanguageByTagParams{ProjectID: pid, Tag: body.Data.TargetLang})
	if err != nil {
		return espresso.JSON[autoTranslateDTO]{}, espresso.ErrNotFound("language not found")
	}
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermTranslationsWrite, auth.Check{ProjectID: pid, LanguageID: targetLang.ID})); err != nil {
		return espresso.JSON[autoTranslateDTO]{}, err
	}
	proj, err := s.store.GetProject(ctx, pid)
	if err != nil || proj.BaseLanguageID.String == "" {
		return espresso.JSON[autoTranslateDTO]{}, espresso.ErrPreconditionFailed("the project has no base language")
	}
	baseID := proj.BaseLanguageID.String
	if targetLang.ID == baseID {
		return espresso.JSON[autoTranslateDTO]{}, espresso.ErrBadRequest("cannot auto-translate the base language")
	}
	baseLang, err := s.store.GetLanguage(ctx, baseID)
	if err != nil {
		return espresso.JSON[autoTranslateDTO]{}, espresso.ErrInternal("base language lookup failed")
	}

	// MT is optional — without it, only exact TM hits are applied.
	var provider mt.Provider
	if cfg, err := s.store.GetMTConfig(ctx, pid); err == nil && cfg.Enabled {
		if apiKey, err := s.openMTKey(cfg); err == nil {
			provider, _ = mt.NewProvider(mt.Config{Provider: cfg.Provider, APIKey: apiKey, Model: cfg.Model})
		}
	}

	limit := body.Data.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	keys, err := s.store.ListKeys(ctx, db.ListKeysParams{ProjectID: pid, Lim: 500, Off: 0})
	if err != nil {
		return espresso.JSON[autoTranslateDTO]{}, espresso.ErrInternal("could not list keys")
	}
	ids := make([]string, len(keys))
	for i, k := range keys {
		ids[i] = k.ID
	}
	byKey := map[string]map[string]db.Translation{}
	if len(ids) > 0 {
		trs, err := s.store.ListTranslationsForKeys(ctx, ids)
		if err != nil {
			return espresso.JSON[autoTranslateDTO]{}, espresso.ErrInternal("could not load translations")
		}
		for _, t := range trs {
			m := byKey[t.KeyID]
			if m == nil {
				m = map[string]db.Translation{}
				byKey[t.KeyID] = m
			}
			m[t.LanguageID] = t
		}
	}

	actor := service.Actor{Kind: db.AuthorKindMt, UserID: auth.FromContext(ctx).UserID}
	out := autoTranslateDTO{TargetLang: body.Data.TargetLang}

	for _, key := range keys {
		if out.Translated >= limit {
			break
		}
		row := byKey[key.ID]
		source := row[baseID].Text.String
		if source == "" {
			continue // no source to translate from
		}
		if cur, ok := row[targetLang.ID]; ok && cur.Text.String != "" && cur.State != db.TranslationStateUntranslated {
			continue // already has a translation
		}
		out.Scanned++

		var text string
		var origin db.TranslationOrigin
		if exact, err := s.store.FindTMExact(ctx, db.FindTMExactParams{
			ProjectID: pid, SourceLang: baseLang.Tag, TargetLang: targetLang.Tag, SourceHash: i18n.SourceHash(source),
		}); err == nil && len(exact) > 0 {
			text, origin = exact[0].TargetText, db.TranslationOriginMachineTm
		} else if provider != nil {
			res, err := mt.GuardedTranslate(ctx, provider, mt.Request{
				Source: source, SourceLang: baseLang.Tag, TargetLang: targetLang.Tag,
				KeyName: key.Name, Description: key.Description.String,
				Glossary: s.buildGlossaryHints(ctx, pid, targetLang.ID, source),
			})
			if err != nil {
				out.Failed++
				continue
			}
			text, origin = res.Text, db.TranslationOriginMachineMt
		} else {
			out.Skipped++ // no TM hit and no MT configured
			continue
		}

		if _, err := service.SetTranslation(ctx, s.store, service.SetTranslationInput{
			Key: key, Language: targetLang, BaseLanguageID: baseID, Text: text,
			Action: service.SetText, MachineOrigin: origin, Actor: actor,
		}); err != nil {
			out.Failed++
			continue
		}
		out.Translated++
		if origin == db.TranslationOriginMachineTm {
			out.FromTM++
		} else {
			out.FromMT++
		}
	}
	return espresso.JSON[autoTranslateDTO]{Data: out}, nil
}
