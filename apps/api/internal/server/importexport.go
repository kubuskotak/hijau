package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/formats"
	"github.com/portierglobal/hijau/apps/api/internal/id"
	"github.com/portierglobal/hijau/apps/api/internal/service"
)

// fileResponse serves a generated file as an attachment.
type fileResponse struct {
	data        []byte
	contentType string
	filename    string
}

func (r fileResponse) WriteResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", r.contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", r.filename))
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(r.data)
	return err
}

type exportQuery struct {
	Format string `query:"format"`
	Lang   string `query:"lang"`
	State  string `query:"state"`
}

// exportTranslations renders one language's strings in the requested format.
func (s *Server) exportTranslations(ctx context.Context, path *extractor.Path[projectPath], q *extractor.Query[exportQuery]) (fileResponse, error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return fileResponse{}, err
	}
	format, ok := formats.Get(q.Data.Format)
	if !ok {
		return fileResponse{}, espresso.ErrBadRequest("unknown format; supported: " + strings.Join(formats.IDs(), ", "))
	}
	proj, err := s.store.GetProject(ctx, pid)
	if err != nil {
		return fileResponse{}, espresso.ErrNotFound("project not found")
	}
	lang, err := s.store.GetLanguageByTag(ctx, db.GetLanguageByTagParams{ProjectID: pid, Tag: q.Data.Lang})
	if err != nil {
		return fileResponse{}, espresso.ErrNotFound("language not found")
	}

	byKey, keys, err := s.loadProjectStrings(ctx, pid)
	if err != nil {
		return fileResponse{}, err
	}
	entries := make([]formats.Entry, 0, len(keys))
	for _, k := range keys {
		t, ok := byKey[k.ID][lang.ID]
		if !ok || !t.Text.Valid || t.Text.String == "" {
			continue
		}
		if q.Data.State != "" && string(t.State) != q.Data.State {
			continue
		}
		entries = append(entries, formats.Entry{Key: k.Name, Value: t.Text.String})
	}

	data, err := format.Marshal(entries)
	if err != nil {
		return fileResponse{}, espresso.ErrInternal("could not render export")
	}
	filename := fmt.Sprintf("%s-%s.%s", proj.Slug, lang.Tag, format.Ext())
	return fileResponse{data: data, contentType: format.ContentType(), filename: filename}, nil
}

type importReq struct {
	Format   string `json:"format"`
	Lang     string `json:"lang"`
	Conflict string `json:"conflict"` // overwrite (default) | keep-existing | only-empty
	Content  string `json:"content"`
	Async    bool   `json:"async"` // when true, enqueue a task and return 202 + taskId
}

type importResultDTO struct {
	TaskID   string   `json:"taskId,omitempty"` // set only on the async (202) path
	Created  int      `json:"created"`
	Updated  int      `json:"updated"`
	Skipped  int      `json:"skipped"`
	Warnings []string `json:"warnings"`
}

// importTranslations parses an uploaded file and upserts keys + translations for
// one language, honouring the conflict policy. Imported strings are stored with
// origin=import. With "async": true it enqueues a task and returns 202 + taskId
// (the UI polls GET /tasks/{id}); otherwise it runs inline and returns the
// result (the default, so the CLI's `hijau push` keeps its synchronous contract).
func (s *Server) importTranslations(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[importReq]) (espresso.JSON[importResultDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermImportExport, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[importResultDTO]{}, err
	}
	lang, proj, entries, err := s.validateImport(ctx, pid, body.Data)
	if err != nil {
		return espresso.JSON[importResultDTO]{}, err
	}
	uid := auth.FromContext(ctx).UserID

	if body.Data.Async {
		t, err := s.enqueue(ctx, pid, db.TaskTypeImport, body.Data, uid)
		if err != nil {
			return espresso.JSON[importResultDTO]{}, espresso.ErrInternal("could not enqueue import")
		}
		return espresso.JSON[importResultDTO]{
			StatusCode: http.StatusAccepted,
			Data:       importResultDTO{TaskID: t.ID, Warnings: []string{}},
		}, nil
	}

	out, err := s.applyImport(ctx, pid, lang, proj, entries, body.Data.Conflict, uid, nil)
	if err != nil {
		return espresso.JSON[importResultDTO]{}, err
	}
	return espresso.JSON[importResultDTO]{Data: out}, nil
}

// validateImport resolves the format, language and project and parses the file.
// Failures map to the right HTTP error on the synchronous path; the worker
// reuses it and turns any error into a failed task.
func (s *Server) validateImport(ctx context.Context, pid string, req importReq) (db.Language, db.Project, []formats.Entry, error) {
	format, ok := formats.Get(req.Format)
	if !ok {
		return db.Language{}, db.Project{}, nil, espresso.ErrBadRequest("unknown format; supported: " + strings.Join(formats.IDs(), ", "))
	}
	lang, err := s.store.GetLanguageByTag(ctx, db.GetLanguageByTagParams{ProjectID: pid, Tag: req.Lang})
	if err != nil {
		return db.Language{}, db.Project{}, nil, espresso.ErrNotFound("language not found")
	}
	proj, err := s.store.GetProject(ctx, pid)
	if err != nil {
		return db.Language{}, db.Project{}, nil, espresso.ErrNotFound("project not found")
	}
	entries, err := format.Unmarshal([]byte(req.Content))
	if err != nil {
		return db.Language{}, db.Project{}, nil, espresso.ErrBadRequest("could not parse file: " + err.Error())
	}
	return lang, proj, entries, nil
}

// applyImport upserts the parsed entries for one language. progress (nullable)
// is called after each entry with (done, total). Per-entry failures become
// warnings and don't abort; a DB error loading existing strings is fatal.
func (s *Server) applyImport(ctx context.Context, pid string, lang db.Language, proj db.Project, entries []formats.Entry, conflict, actorUserID string, progress func(done, total int)) (importResultDTO, error) {
	if conflict == "" {
		conflict = "overwrite"
	}
	byKey, keys, err := s.loadProjectStrings(ctx, pid)
	if err != nil {
		return importResultDTO{}, err
	}
	byName := map[string]db.TranslationKey{}
	for _, k := range keys {
		byName[k.Name] = k
	}

	actor := service.Actor{Kind: db.AuthorKindUser, UserID: actorUserID}
	out := importResultDTO{Warnings: []string{}}
	total := len(entries)

	for i, e := range entries {
		key, exists := byName[e.Key]
		if !exists {
			created, err := s.store.CreateKey(ctx, db.CreateKeyParams{ID: id.New(), ProjectID: pid, Name: e.Key})
			if err != nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("%s: could not create key", e.Key))
				if progress != nil {
					progress(i+1, total)
				}
				continue
			}
			key = created
			byName[e.Key] = created
		}

		existing := byKey[key.ID][lang.ID]
		hadText := existing.Text.Valid && existing.Text.String != ""
		switch conflict {
		case "keep-existing":
			if hadText {
				out.Skipped++
				if progress != nil {
					progress(i+1, total)
				}
				continue
			}
		case "only-empty":
			if hadText && existing.State != db.TranslationStateUntranslated {
				out.Skipped++
				if progress != nil {
					progress(i+1, total)
				}
				continue
			}
		}

		if _, err := service.SetTranslation(ctx, s.store, service.SetTranslationInput{
			Key: key, Language: lang, BaseLanguageID: proj.BaseLanguageID.String, Text: e.Value,
			Action: service.SetText, Origin: db.TranslationOriginImport, Actor: actor,
		}); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("%s: %v", e.Key, err))
			if progress != nil {
				progress(i+1, total)
			}
			continue
		}
		if hadText {
			out.Updated++
		} else {
			out.Created++
		}
		if progress != nil {
			progress(i+1, total)
		}
	}
	return out, nil
}

// runImportTask is the worker entry point for a queued import task: it re-runs
// validation (mapping failures to a failed task) then applies the import with
// live progress.
func (s *Server) runImportTask(ctx context.Context, t db.Task) (any, error) {
	var req importReq
	if err := json.Unmarshal(t.Payload, &req); err != nil {
		return nil, fmt.Errorf("bad import payload: %w", err)
	}
	pid := t.ProjectID.String
	lang, proj, entries, err := s.validateImport(ctx, pid, req)
	if err != nil {
		return nil, err
	}
	return s.applyImport(ctx, pid, lang, proj, entries, req.Conflict, t.CreatedBy.String, s.taskProgress(t))
}

// loadProjectStrings returns the project's keys and a keyId->langId->translation
// map (up to 1000 keys).
func (s *Server) loadProjectStrings(ctx context.Context, pid string) (map[string]map[string]db.Translation, []db.TranslationKey, error) {
	keys, err := s.store.ListKeys(ctx, db.ListKeysParams{ProjectID: pid, Lim: 1000, Off: 0})
	if err != nil {
		return nil, nil, espresso.ErrInternal("could not list keys")
	}
	ids := make([]string, len(keys))
	for i, k := range keys {
		ids[i] = k.ID
	}
	byKey := map[string]map[string]db.Translation{}
	if len(ids) > 0 {
		trs, err := s.store.ListTranslationsForKeys(ctx, ids)
		if err != nil {
			return nil, nil, espresso.ErrInternal("could not load translations")
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
	return byKey, keys, nil
}
