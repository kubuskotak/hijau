package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// Minimal REST client for the Hijau API. Mirrors the DTO field names so JSON
// decodes directly.

type Project struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	BaseLanguageID string `json:"baseLanguageId"`
}

type Language struct {
	ID    string `json:"id"`
	Tag   string `json:"tag"`
	Name  string `json:"name"`
	IsRtl bool   `json:"isRtl"`
}

type Key struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Translation struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	State string `json:"state"`
	SubID int64  `json:"subId"`
}

type EditorRow struct {
	Key
	Translations map[string]Translation `json:"translations"`
}

type EditorFeed struct {
	Keys  []EditorRow `json:"keys"`
	Total int64       `json:"total"`
}

type apiError struct {
	Err struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type Client struct {
	base  string
	token string
	http  *http.Client
}

func newClient(cfg Config) *Client {
	base := strings.TrimRight(cfg.APIURL, "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return &Client{base: base + "/api/v1", token: cfg.Token, http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) do(method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		var ae apiError
		if json.Unmarshal(data, &ae) == nil && ae.Err.Message != "" {
			return fmt.Errorf("%s (%d %s)", ae.Err.Message, res.StatusCode, ae.Err.Code)
		}
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (c *Client) listProjects() ([]Project, error) {
	var p []Project
	return p, c.do("GET", "/projects", nil, &p)
}
func (c *Client) getProject(id string) (Project, error) {
	var p Project
	return p, c.do("GET", "/projects/"+id, nil, &p)
}
func (c *Client) listLanguages(pid string) ([]Language, error) {
	var l []Language
	return l, c.do("GET", "/projects/"+pid+"/languages", nil, &l)
}
func (c *Client) editorFeed(pid string, limit int) (EditorFeed, error) {
	var f EditorFeed
	return f, c.do("GET", fmt.Sprintf("/projects/%s/editor?limit=%d", pid, limit), nil, &f)
}
func (c *Client) createKey(pid, name string) (Key, error) {
	var k Key
	return k, c.do("POST", "/projects/"+pid+"/keys", map[string]string{"name": name}, &k)
}
func (c *Client) setTranslation(pid, keyID, lang, text string) (Translation, error) {
	var t Translation
	return t, c.do("PUT", fmt.Sprintf("/projects/%s/keys/%s/translations/%s", pid, keyID, lang), map[string]string{"text": text}, &t)
}

// login authenticates with email+password (session cookie) and mints a PAT.
func login(apiURL, email, password string) (string, error) {
	base := strings.TrimRight(apiURL, "/") + "/api/v1"
	jar, _ := cookiejar.New(nil)
	hc := &http.Client{Timeout: 30 * time.Second, Jar: jar}

	post := func(path string, body any, out any) error {
		buf, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", base+path, bytes.NewReader(buf))
		req.Header.Set("Content-Type", "application/json")
		res, err := hc.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if res.StatusCode >= 400 {
			var ae apiError
			if json.Unmarshal(data, &ae) == nil && ae.Err.Message != "" {
				return fmt.Errorf("%s", ae.Err.Message)
			}
			return fmt.Errorf("HTTP %d", res.StatusCode)
		}
		if out != nil {
			return json.Unmarshal(data, out)
		}
		return nil
	}

	if err := post("/auth/login", map[string]string{"email": email, "password": password}, nil); err != nil {
		return "", fmt.Errorf("sign-in failed: %w", err)
	}
	var tok struct {
		Token string `json:"token"`
	}
	if err := post("/me/tokens", map[string]string{"name": "hijau-cli"}, &tok); err != nil {
		return "", fmt.Errorf("could not mint token: %w", err)
	}
	return tok.Token, nil
}

// ensure url is well-formed early so errors are friendly.
func validateURL(s string) error {
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid --api url %q", s)
	}
	return nil
}
