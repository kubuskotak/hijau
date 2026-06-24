package server

import (
	"context"
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
}

type importResultDTO struct {
	Created  int      `json:"created"`
	Updated  int      `json:"updated"`
	Skipped  int      `json:"skipped"`
	Warnings []string `json:"warnings"`
}

// importTranslations parses an uploaded file and upserts keys + translations for
// one language, honouring the conflict policy. Imported strings are stored with
// origin=import.
func (s *Server) importTranslations(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[importReq]) (espresso.JSON[importResultDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermImportExport, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[importResultDTO]{}, err
	}
	format, ok := formats.Get(body.Data.Format)
	if !ok {
		return espresso.JSON[importResultDTO]{}, espresso.ErrBadRequest("unknown format; supported: " + strings.Join(formats.IDs(), ", "))
	}
	lang, err := s.store.GetLanguageByTag(ctx, db.GetLanguageByTagParams{ProjectID: pid, Tag: body.Data.Lang})
	if err != nil {
		return espresso.JSON[importResultDTO]{}, espresso.ErrNotFound("language not found")
	}
	proj, err := s.store.GetProject(ctx, pid)
	if err != nil {
		return espresso.JSON[importResultDTO]{}, espresso.ErrNotFound("project not found")
	}
	entries, err := format.Unmarshal([]byte(body.Data.Content))
	if err != nil {
		return espresso.JSON[importResultDTO]{}, espresso.ErrBadRequest("could not parse file: " + err.Error())
	}
	conflict := body.Data.Conflict
	if conflict == "" {
		conflict = "overwrite"
	}

	byKey, keys, err := s.loadProjectStrings(ctx, pid)
	if err != nil {
		return espresso.JSON[importResultDTO]{}, err
	}
	byName := map[string]db.TranslationKey{}
	for _, k := range keys {
		byName[k.Name] = k
	}

	actor := service.Actor{Kind: db.AuthorKindUser, UserID: auth.FromContext(ctx).UserID}
	out := importResultDTO{Warnings: []string{}}

	for _, e := range entries {
		key, exists := byName[e.Key]
		if !exists {
			created, err := s.store.CreateKey(ctx, db.CreateKeyParams{ID: id.New(), ProjectID: pid, Name: e.Key})
			if err != nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("%s: could not create key", e.Key))
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
				continue
			}
		case "only-empty":
			if hadText && existing.State != db.TranslationStateUntranslated {
				out.Skipped++
				continue
			}
		}

		if _, err := service.SetTranslation(ctx, s.store, service.SetTranslationInput{
			Key: key, Language: lang, BaseLanguageID: proj.BaseLanguageID.String, Text: e.Value,
			Action: service.SetText, Origin: db.TranslationOriginImport, Actor: actor,
		}); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("%s: %v", e.Key, err))
			continue
		}
		if hadText {
			out.Updated++
		} else {
			out.Created++
		}
	}
	return espresso.JSON[importResultDTO]{Data: out}, nil
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
