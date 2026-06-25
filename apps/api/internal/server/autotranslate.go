package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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
	Async      bool   `json:"async"` // when true, enqueue a task and return 202 + taskId
}

type autoTranslateDTO struct {
	TaskID     string `json:"taskId,omitempty"` // set only on the async (202) path
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
	targetLang, err := s.lookupTargetLang(ctx, pid, body.Data)
	if err != nil {
		return espresso.JSON[autoTranslateDTO]{}, err
	}
	// Authorize before touching project/base-language config so an unauthorized
	// caller gets 403 — not a 412/400 that leaks whether a base language is set.
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermTranslationsWrite, auth.Check{ProjectID: pid, LanguageID: targetLang.ID})); err != nil {
		return espresso.JSON[autoTranslateDTO]{}, err
	}
	baseLang, limit, err := s.resolveBaseAndLimit(ctx, pid, targetLang, body.Data)
	if err != nil {
		return espresso.JSON[autoTranslateDTO]{}, err
	}
	uid := auth.FromContext(ctx).UserID

	if body.Data.Async {
		t, err := s.enqueue(ctx, pid, db.TaskTypeAutoTranslate, body.Data, uid)
		if err != nil {
			return espresso.JSON[autoTranslateDTO]{}, espresso.ErrInternal("could not enqueue auto-translate")
		}
		return espresso.JSON[autoTranslateDTO]{
			StatusCode: http.StatusAccepted,
			Data:       autoTranslateDTO{TaskID: t.ID, TargetLang: targetLang.Tag},
		}, nil
	}

	out, err := s.runAutoTranslate(ctx, pid, baseLang, targetLang, limit, uid, nil)
	if err != nil {
		return espresso.JSON[autoTranslateDTO]{}, err
	}
	return espresso.JSON[autoTranslateDTO]{Data: out}, nil
}

// lookupTargetLang resolves the target language (the part needed to authorize).
func (s *Server) lookupTargetLang(ctx context.Context, pid string, req autoTranslateReq) (db.Language, error) {
	if req.TargetLang == "" {
		return db.Language{}, espresso.ErrBadRequest("targetLang is required")
	}
	targetLang, err := s.store.GetLanguageByTag(ctx, db.GetLanguageByTagParams{ProjectID: pid, Tag: req.TargetLang})
	if err != nil {
		return db.Language{}, espresso.ErrNotFound("language not found")
	}
	return targetLang, nil
}

// resolveBaseAndLimit validates the project's base language (vs the target) and
// the effective limit. Runs after authorization (handler) — the worker calls it
// directly since enqueue already authorized the request.
func (s *Server) resolveBaseAndLimit(ctx context.Context, pid string, targetLang db.Language, req autoTranslateReq) (db.Language, int, error) {
	proj, err := s.store.GetProject(ctx, pid)
	if err != nil || proj.BaseLanguageID.String == "" {
		return db.Language{}, 0, espresso.ErrPreconditionFailed("the project has no base language")
	}
	baseID := proj.BaseLanguageID.String
	if targetLang.ID == baseID {
		return db.Language{}, 0, espresso.ErrBadRequest("cannot auto-translate the base language")
	}
	baseLang, err := s.store.GetLanguage(ctx, baseID)
	if err != nil {
		return db.Language{}, 0, espresso.ErrInternal("base language lookup failed")
	}
	limit := req.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return baseLang, limit, nil
}

// runAutoTranslate fills untranslated keys for the target language (TM exact hit,
// else MT through the ICU guard). progress (nullable) is called after each
// scanned candidate with (done, total). Safe to call from a worker goroutine.
func (s *Server) runAutoTranslate(ctx context.Context, pid string, baseLang, targetLang db.Language, limit int, actorUserID string, progress func(done, total int)) (autoTranslateDTO, error) {
	baseID := baseLang.ID
	out := autoTranslateDTO{TargetLang: targetLang.Tag}

	// MT is optional — without it, only exact TM hits are applied.
	var provider mt.Provider
	if cfg, err := s.store.GetMTConfig(ctx, pid); err == nil && cfg.Enabled {
		if apiKey, err := s.openMTKey(cfg); err == nil {
			provider, _ = mt.NewProvider(mt.Config{Provider: cfg.Provider, APIKey: apiKey, Model: cfg.Model})
		}
	}

	keys, err := s.store.ListKeys(ctx, db.ListKeysParams{ProjectID: pid, Lim: 500, Off: 0})
	if err != nil {
		return out, espresso.ErrInternal("could not list keys")
	}
	ids := make([]string, len(keys))
	for i, k := range keys {
		ids[i] = k.ID
	}
	byKey := map[string]map[string]db.Translation{}
	if len(ids) > 0 {
		trs, err := s.store.ListTranslationsForKeys(ctx, ids)
		if err != nil {
			return out, espresso.ErrInternal("could not load translations")
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

	actor := service.Actor{Kind: db.AuthorKindMt, UserID: actorUserID}
	total := len(keys)

	for i, key := range keys {
		if progress != nil {
			progress(i+1, total)
		}
		if out.Translated >= limit {
			continue
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
			Action: service.SetText, Origin: origin, Actor: actor,
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
	return out, nil
}

// runAutoTranslateTask is the worker entry point for a queued auto-translate task.
func (s *Server) runAutoTranslateTask(ctx context.Context, t db.Task) (any, error) {
	var req autoTranslateReq
	if err := json.Unmarshal(t.Payload, &req); err != nil {
		return nil, fmt.Errorf("bad auto-translate payload: %w", err)
	}
	pid := t.ProjectID.String
	targetLang, err := s.lookupTargetLang(ctx, pid, req)
	if err != nil {
		return nil, err
	}
	baseLang, limit, err := s.resolveBaseAndLimit(ctx, pid, targetLang, req)
	if err != nil {
		return nil, err
	}
	return s.runAutoTranslate(ctx, pid, baseLang, targetLang, limit, t.CreatedBy.String, s.taskProgress(t))
}
