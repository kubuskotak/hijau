package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/id"
)

const maxScreenshotBytes = 8 << 20 // 8 MiB

// imageResponse serves raw image bytes with an explicit content type — the
// built-in JSON/Text responses can't, but any type implementing IntoResponse
// works as a handler return value.
type imageResponse struct {
	data        []byte
	contentType string
}

func (r imageResponse) WriteResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", r.contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(r.data)
	return err
}

type uploadRegionReq struct {
	SubID int64 `json:"subId"`
	X     int32 `json:"x"`
	Y     int32 `json:"y"`
	W     int32 `json:"w"`
	H     int32 `json:"h"`
}

type uploadScreenshotReq struct {
	Image   string            `json:"image"` // data URL or bare base64 PNG
	Name    string            `json:"name"`
	Width   int32             `json:"width"`
	Height  int32             `json:"height"`
	Regions []uploadRegionReq `json:"regions"`
}

type regionDTO struct {
	ID    string `json:"id"`
	KeyID string `json:"keyId"`
	SubID int64  `json:"subId,omitempty"`
	X     int32  `json:"x"`
	Y     int32  `json:"y"`
	W     int32  `json:"w"`
	H     int32  `json:"h"`
}

type screenshotDTO struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Width     int32       `json:"width"`
	Height    int32       `json:"height"`
	ImageURL  string      `json:"imageUrl"`
	CreatedAt string      `json:"createdAt"`
	Regions   []regionDTO `json:"regions"`
}

func imageURL(pid, sid string) string {
	return fmt.Sprintf("/api/v1/projects/%s/screenshots/%s/image", pid, sid)
}

// decodeImage strips an optional `data:...;base64,` prefix and decodes.
func decodeImage(s string) ([]byte, error) {
	if strings.HasPrefix(s, "data:") {
		if i := strings.Index(s, ","); i >= 0 {
			s = s[i+1:]
		}
	}
	return base64.StdEncoding.DecodeString(strings.TrimSpace(s))
}

// uploadScreenshot stores a captured image and maps the given regions to the
// translations they cover (resolved from the marker sub_id). Requires
// screenshots:write — which the unlocked in-context editor token carries.
func (s *Server) uploadScreenshot(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[uploadScreenshotReq]) (espresso.JSON[screenshotDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermScreenshotWrite, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[screenshotDTO]{}, err
	}
	data, err := decodeImage(body.Data.Image)
	if err != nil || len(data) == 0 {
		return espresso.JSON[screenshotDTO]{}, espresso.ErrBadRequest("invalid image data")
	}
	if len(data) > maxScreenshotBytes {
		return espresso.JSON[screenshotDTO]{}, espresso.ErrBadRequest("image too large (max 8MB)")
	}

	sid := id.New()
	storageKey := sid + ".png"
	if err := s.storage.Put(storageKey, data); err != nil {
		return espresso.JSON[screenshotDTO]{}, espresso.ErrInternal("could not store image")
	}

	var createdBy pgtype.Text
	if uid := auth.FromContext(ctx).UserID; uid != "" {
		createdBy = pgText(uid)
	}

	// The screenshot row and its regions are written in one transaction so a
	// region failure can't leave an orphaned screenshot; the blob is deleted if
	// the transaction rolls back.
	var out screenshotDTO
	txErr := s.store.WithTx(ctx, func(q *db.Queries) error {
		sc, err := q.CreateScreenshot(ctx, db.CreateScreenshotParams{
			ID: sid, ProjectID: pid, StorageKey: storageKey, Name: body.Data.Name,
			Width: body.Data.Width, Height: body.Data.Height, CreatedBy: createdBy,
		})
		if err != nil {
			return err
		}
		out = screenshotDTO{
			ID: sc.ID, Name: sc.Name, Width: sc.Width, Height: sc.Height,
			ImageURL: imageURL(pid, sc.ID), CreatedAt: sc.CreatedAt.Time.UTC().Format(time.RFC3339),
		}
		for _, r := range body.Data.Regions {
			tr, err := q.GetTranslationBySubID(ctx, pgtype.Int8{Int64: r.SubID, Valid: true})
			if err != nil {
				continue // unknown sub_id — skip the region rather than fail the upload
			}
			key, err := q.GetKey(ctx, tr.KeyID)
			if err != nil || key.ProjectID != pid {
				continue // region points outside this project
			}
			reg, err := q.CreateScreenshotRegion(ctx, db.CreateScreenshotRegionParams{
				ID: id.New(), ScreenshotID: sc.ID, KeyID: key.ID, TranslationID: pgText(tr.ID),
				X: r.X, Y: r.Y, W: r.W, H: r.H,
			})
			if err != nil {
				return err
			}
			out.Regions = append(out.Regions, regionDTO{ID: reg.ID, KeyID: key.ID, SubID: r.SubID, X: reg.X, Y: reg.Y, W: reg.W, H: reg.H})
		}
		return nil
	})
	if txErr != nil {
		_ = s.storage.Delete(storageKey) // best-effort: don't leave an orphaned blob
		return espresso.JSON[screenshotDTO]{}, espresso.ErrInternal("could not save screenshot")
	}
	return espresso.JSON[screenshotDTO]{StatusCode: http.StatusCreated, Data: out}, nil
}

type screenshotPath struct {
	PID string `path:"pid"`
	SID string `path:"sid"`
}

func (s *Server) serveScreenshotImage(ctx context.Context, path *extractor.Path[screenshotPath]) (imageResponse, error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return imageResponse{}, err
	}
	sc, err := s.store.GetScreenshot(ctx, path.Data.SID)
	if err != nil || sc.ProjectID != pid {
		return imageResponse{}, espresso.ErrNotFound("screenshot not found")
	}
	data, err := s.storage.Get(sc.StorageKey)
	if err != nil {
		return imageResponse{}, espresso.ErrNotFound("image not found")
	}
	return imageResponse{data: data, contentType: "image/png"}, nil
}

// listKeyScreenshots returns the screenshots a key appears in, each with the
// regions that highlight that key — backing the editor's screenshot gallery.
func (s *Server) listKeyScreenshots(ctx context.Context, path *extractor.Path[keyPath]) (espresso.JSON[[]screenshotDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectRead, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[[]screenshotDTO]{}, err
	}
	key, err := s.store.GetKey(ctx, path.Data.KID)
	if err != nil || key.ProjectID != pid {
		return espresso.JSON[[]screenshotDTO]{}, espresso.ErrNotFound("key not found")
	}
	rows, err := s.store.ListKeyScreenshotRegions(ctx, key.ID)
	if err != nil {
		return espresso.JSON[[]screenshotDTO]{}, espresso.ErrInternal("could not load screenshots")
	}

	order := make([]string, 0)
	byID := make(map[string]*screenshotDTO)
	for _, row := range rows {
		sc, r := row.Screenshot, row.ScreenshotRegion
		d, ok := byID[sc.ID]
		if !ok {
			d = &screenshotDTO{
				ID: sc.ID, Name: sc.Name, Width: sc.Width, Height: sc.Height,
				ImageURL: imageURL(pid, sc.ID), CreatedAt: sc.CreatedAt.Time.UTC().Format(time.RFC3339),
			}
			byID[sc.ID] = d
			order = append(order, sc.ID)
		}
		d.Regions = append(d.Regions, regionDTO{ID: r.ID, KeyID: r.KeyID, X: r.X, Y: r.Y, W: r.W, H: r.H})
	}
	out := make([]screenshotDTO, 0, len(order))
	for _, sid := range order {
		out = append(out, *byID[sid])
	}
	return espresso.JSON[[]screenshotDTO]{Data: out}, nil
}
