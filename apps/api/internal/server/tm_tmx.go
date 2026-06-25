package server

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/formats"
	"github.com/portierglobal/hijau/apps/api/internal/i18n"
	"github.com/portierglobal/hijau/apps/api/internal/id"
)

type importTMXReq struct {
	Content string `json:"content"`
}

type importTMXResultDTO struct {
	Imported int      `json:"imported"` // fresh segments inserted
	Skipped  int      `json:"skipped"`  // duplicates (already in the TM) or empty units
	Warnings []string `json:"warnings"`
}

// importTMX ingests a TMX file into the project's translation memory. Each
// bilingual unit is upserted (idempotent — duplicates are skipped). Segments are
// stored with origin "tmx" and no originating key; they're immediately usable by
// TM suggestions and auto-translate. TMX langs not registered on the project are
// imported but flagged (they won't surface in suggestions until the lang exists).
func (s *Server) importTMX(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[importTMXReq]) (espresso.JSON[importTMXResultDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermImportExport, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[importTMXResultDTO]{}, err
	}
	if _, err := s.store.GetProject(ctx, pid); err != nil {
		return espresso.JSON[importTMXResultDTO]{}, espresso.ErrNotFound("project not found")
	}
	units, skippedTU, err := formats.UnmarshalTMX([]byte(body.Data.Content))
	if err != nil {
		return espresso.JSON[importTMXResultDTO]{}, espresso.ErrBadRequest("could not parse TMX: " + err.Error())
	}

	// Known project langs — to warn about segments whose tags aren't configured.
	// On a lookup failure, skip the check rather than flag every tag as unknown.
	known := map[string]bool{}
	langsKnown := false
	if langs, err := s.store.ListLanguages(ctx, pid); err == nil {
		langsKnown = true
		for _, l := range langs {
			known[l.Tag] = true
		}
	}

	out := importTMXResultDTO{Warnings: []string{}}
	out.Skipped = skippedTU // TUs the codec dropped (fewer than 2 variants / no declared source)
	unknown := map[string]bool{}
	flattened := 0
	err = s.store.WithTx(ctx, func(q *db.Queries) error {
		for _, u := range units {
			if u.SourceText == "" || u.TargetText == "" || u.SourceLang == "" || u.TargetLang == "" {
				out.Skipped++
				continue
			}
			if u.Flattened {
				flattened++
			}
			if langsKnown {
				if !known[u.SourceLang] {
					unknown[u.SourceLang] = true
				}
				if !known[u.TargetLang] {
					unknown[u.TargetLang] = true
				}
			}
			_, err := q.InsertTMSegmentReturning(ctx, db.InsertTMSegmentReturningParams{
				ID: id.New(), ProjectID: pid,
				SourceLang: u.SourceLang, TargetLang: u.TargetLang,
				SourceText: u.SourceText, TargetText: u.TargetText,
				SourceHash: i18n.SourceHash(u.SourceText),
				KeyID:      pgtype.Text{}, Origin: "tmx",
			})
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				out.Skipped++ // ON CONFLICT DO NOTHING — already in the TM
			case err != nil:
				return err
			default:
				out.Imported++
			}
		}
		return nil
	})
	if err != nil {
		return espresso.JSON[importTMXResultDTO]{}, espresso.ErrInternal("could not import TMX")
	}

	if flattened > 0 {
		out.Warnings = append(out.Warnings,
			fmt.Sprintf("%d segment(s) contained inline formatting tags that were flattened to plain text; placeholders/formatting may be incomplete", flattened))
	}
	for _, tag := range sortedKeys(unknown) {
		out.Warnings = append(out.Warnings,
			fmt.Sprintf("language %q isn't configured on this project; its segments won't appear in suggestions until you add it", tag))
	}
	return espresso.JSON[importTMXResultDTO]{Data: out}, nil
}

type exportTMXQuery struct {
	SourceLang string `query:"sourceLang"`
	TargetLang string `query:"targetLang"`
}

// exportTMX dumps the project's translation memory as a TMX 1.4 file, optionally
// narrowed to a source/target language pair.
func (s *Server) exportTMX(ctx context.Context, path *extractor.Path[projectPath], q *extractor.Query[exportTMXQuery]) (fileResponse, error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return fileResponse{}, err
	}
	proj, err := s.store.GetProject(ctx, pid)
	if err != nil {
		return fileResponse{}, espresso.ErrNotFound("project not found")
	}
	rows, err := s.store.ListTMSegments(ctx, db.ListTMSegmentsParams{
		ProjectID: pid, SourceLang: q.Data.SourceLang, TargetLang: q.Data.TargetLang,
	})
	if err != nil {
		return fileResponse{}, espresso.ErrInternal("could not load translation memory")
	}
	units := make([]formats.TMXUnit, 0, len(rows))
	for _, r := range rows {
		units = append(units, formats.TMXUnit{
			SourceLang: r.SourceLang, SourceText: r.SourceText,
			TargetLang: r.TargetLang, TargetText: r.TargetText,
		})
	}

	// Header srclang: the requested source, else the project's base language tag,
	// else the TMX "varies" sentinel.
	srcLang := q.Data.SourceLang
	if srcLang == "" && proj.BaseLanguageID.String != "" {
		if bl, err := s.store.GetLanguage(ctx, proj.BaseLanguageID.String); err == nil {
			srcLang = bl.Tag
		}
	}
	data, err := formats.MarshalTMX(srcLang, units)
	if err != nil {
		return fileResponse{}, espresso.ErrInternal("could not render TMX")
	}
	return fileResponse{data: data, contentType: "application/x-tmx+xml", filename: proj.Slug + "-tm.tmx"}, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
