package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/suryakencana007/espresso/v2"
	"github.com/suryakencana007/espresso/v2/extractor"

	"github.com/portierglobal/hijau/apps/api/internal/auth"
	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/id"
)

type createWebhookReq struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

type webhookDTO struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Active    bool     `json:"active"`
	CreatedAt string   `json:"createdAt"`
	Secret    string   `json:"secret,omitempty"` // returned only once, on create
}

func toWebhookDTO(w db.Webhook) webhookDTO {
	return webhookDTO{
		ID: w.ID, URL: w.Url, Events: w.Events, Active: w.Active,
		CreatedAt: w.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
}

func (s *Server) listWebhooks(ctx context.Context, path *extractor.Path[projectPath]) (espresso.JSON[[]webhookDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectAdmin, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[[]webhookDTO]{}, err
	}
	hooks, err := s.store.ListWebhooks(ctx, pid)
	if err != nil {
		return espresso.JSON[[]webhookDTO]{}, espresso.ErrInternal("could not list webhooks")
	}
	out := make([]webhookDTO, 0, len(hooks))
	for _, h := range hooks {
		out = append(out, toWebhookDTO(h)) // never includes the secret
	}
	return espresso.JSON[[]webhookDTO]{Data: out}, nil
}

func (s *Server) createWebhook(ctx context.Context, path *extractor.Path[projectPath], body *espresso.JSON[createWebhookReq]) (espresso.JSON[webhookDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectAdmin, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[webhookDTO]{}, err
	}
	raw := strings.TrimSpace(body.Data.URL)
	if u, err := url.Parse(raw); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return espresso.JSON[webhookDTO]{}, espresso.ErrBadRequest("url must be a valid http(s) URL")
	}
	if s.cipher == nil {
		return espresso.JSON[webhookDTO]{}, espresso.ErrInternal("server has no HIJAU_ENCRYPTION_KEY; cannot store the signing secret")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return espresso.JSON[webhookDTO]{}, espresso.ErrInternal("could not generate secret")
	}
	secret := hex.EncodeToString(buf)
	sealed, err := s.cipher.Seal([]byte(secret))
	if err != nil {
		return espresso.JSON[webhookDTO]{}, espresso.ErrInternal("could not seal secret")
	}
	events := body.Data.Events
	if events == nil {
		events = []string{}
	}
	w, err := s.store.CreateWebhook(ctx, db.CreateWebhookParams{
		ID: id.New(), ProjectID: pid, Url: raw, Secret: sealed, Events: events, Active: true,
	})
	if err != nil {
		return espresso.JSON[webhookDTO]{}, espresso.ErrInternal("could not create webhook")
	}
	dto := toWebhookDTO(w)
	dto.Secret = secret // shown exactly once
	return espresso.JSON[webhookDTO]{StatusCode: http.StatusCreated, Data: dto}, nil
}

type webhookPath struct {
	PID string `path:"pid"`
	WID string `path:"wid"`
}

func (s *Server) deleteWebhook(ctx context.Context, path *extractor.Path[webhookPath]) (espresso.JSON[okDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectAdmin, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[okDTO]{}, err
	}
	w, err := s.store.GetWebhook(ctx, path.Data.WID)
	if err != nil || w.ProjectID != pid {
		return espresso.JSON[okDTO]{}, espresso.ErrNotFound("webhook not found")
	}
	if err := s.store.DeleteWebhook(ctx, w.ID); err != nil {
		return espresso.JSON[okDTO]{}, espresso.ErrInternal("could not delete webhook")
	}
	return espresso.JSON[okDTO]{Data: okDTO{OK: true}}, nil
}

type deliveryDTO struct {
	ID         string `json:"id"`
	Event      string `json:"event"`
	StatusCode int32  `json:"statusCode"`
	Success    bool   `json:"success"`
	Error      string `json:"error"`
	CreatedAt  string `json:"createdAt"`
}

func (s *Server) listWebhookDeliveries(ctx context.Context, path *extractor.Path[webhookPath]) (espresso.JSON[[]deliveryDTO], error) {
	pid := path.Data.PID
	if err := authErr(auth.Authorize(ctx, s.store, auth.PermProjectAdmin, auth.Check{ProjectID: pid})); err != nil {
		return espresso.JSON[[]deliveryDTO]{}, err
	}
	w, err := s.store.GetWebhook(ctx, path.Data.WID)
	if err != nil || w.ProjectID != pid {
		return espresso.JSON[[]deliveryDTO]{}, espresso.ErrNotFound("webhook not found")
	}
	rows, err := s.store.ListWebhookDeliveries(ctx, w.ID)
	if err != nil {
		return espresso.JSON[[]deliveryDTO]{}, espresso.ErrInternal("could not load deliveries")
	}
	out := make([]deliveryDTO, 0, len(rows))
	for _, d := range rows {
		out = append(out, deliveryDTO{
			ID: d.ID, Event: d.Event, StatusCode: d.StatusCode, Success: d.Success, Error: d.Error,
			CreatedAt: d.CreatedAt.Time.UTC().Format(time.RFC3339),
		})
	}
	return espresso.JSON[[]deliveryDTO]{Data: out}, nil
}

// --- dispatch ---

type webhookPayload struct {
	Event     string `json:"event"`
	ProjectID string `json:"projectId"`
	Key       string `json:"key"`
	Language  string `json:"language"`
	Text      string `json:"text"`
	State     string `json:"state"`
	Timestamp string `json:"timestamp"`
}

var webhookClient = &http.Client{Timeout: 10 * time.Second}

// dispatchWebhooks delivers an event to every active, subscribed webhook for a
// project, each in its own goroutine (fire-and-forget; failures are logged, not
// surfaced to the triggering request).
func (s *Server) dispatchWebhooks(projectID, event string, p webhookPayload) {
	hooks, err := s.store.ListActiveWebhooks(context.Background(), projectID)
	if err != nil || len(hooks) == 0 {
		return
	}
	body, err := json.Marshal(p)
	if err != nil {
		return
	}
	for _, h := range hooks {
		if !subscribed(h.Events, event) {
			continue
		}
		secret, err := s.openSecret(h.Secret)
		if err != nil {
			continue
		}
		go s.deliverWebhook(h.ID, h.Url, event, secret, body)
	}
}

func subscribed(events []string, event string) bool {
	if len(events) == 0 {
		return true // empty = all events
	}
	for _, e := range events {
		if e == event {
			return true
		}
	}
	return false
}

func (s *Server) openSecret(sealed []byte) ([]byte, error) {
	if s.cipher == nil {
		return nil, errors.New("no encryption key")
	}
	return s.cipher.Open(sealed)
}

func (s *Server) deliverWebhook(webhookID, url, event string, secret, body []byte) {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	var status int
	var ok bool
	var errMsg string
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			errMsg = err.Error()
			break
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hijau-Event", event)
		req.Header.Set("X-Hijau-Signature", sig)
		res, err := webhookClient.Do(req)
		if err != nil {
			errMsg = err.Error()
		} else {
			status = res.StatusCode
			res.Body.Close()
			if status >= 200 && status < 300 {
				ok, errMsg = true, ""
				break
			}
			errMsg = fmt.Sprintf("HTTP %d", status)
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
		}
	}
	_ = s.store.InsertWebhookDelivery(context.Background(), db.InsertWebhookDeliveryParams{
		ID: id.New(), WebhookID: webhookID, Event: event, StatusCode: int32(status), Success: ok, Error: errMsg,
	})
}
