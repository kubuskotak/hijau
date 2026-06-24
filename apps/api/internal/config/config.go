// Package config loads runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// LoadDotenv best-effort loads a .env file from the working directory or a few
// parent directories. Each candidate is loaded independently so a missing file
// doesn't short-circuit the rest (godotenv.Load aborts on the first failure).
// Existing environment variables are never overridden.
func LoadDotenv() {
	for _, f := range []string{".env", "../.env", "../../.env", "../../../.env"} {
		_ = godotenv.Load(f)
	}
}

type Config struct {
	DatabaseURL   string
	Port          string
	PublicURL     string
	SessionSecret string
	EncryptionKey string
	CORSOrigins   []string
	Storage       string
	StorageDir    string
	WebDir        string // built SPA dir to serve; empty = API only (dev uses Vite)
	Seed          bool
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	c := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		Port:          getenv("PORT", "8080"),
		PublicURL:     getenv("PUBLIC_URL", "http://localhost:8080"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
		EncryptionKey: os.Getenv("HIJAU_ENCRYPTION_KEY"),
		Storage:       getenv("HIJAU_STORAGE", "fs"),
		StorageDir:    getenv("HIJAU_STORAGE_DIR", "./data/screenshots"),
		WebDir:        os.Getenv("HIJAU_WEB_DIR"),
		Seed:          os.Getenv("HIJAU_SEED") == "1",
	}
	if raw := os.Getenv("CORS_ORIGINS"); raw != "" {
		for o := range strings.SplitSeq(raw, ",") {
			if o = strings.TrimSpace(o); o != "" {
				c.CORSOrigins = append(c.CORSOrigins, o)
			}
		}
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
