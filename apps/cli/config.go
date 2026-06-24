package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Config is the persisted CLI state: where the instance is, the PAT to use, and
// an optional default project so most commands don't need --project.
type Config struct {
	APIURL         string `json:"apiUrl"`
	Token          string `json:"token"`
	DefaultProject string `json:"defaultProject,omitempty"`
}

func configPath() (string, error) {
	if p := os.Getenv("HIJAU_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hijau", "config.json"), nil
}

// loadConfig reads the config file. A missing file is not an error — it returns
// a zero Config so `login` can create one.
func loadConfig() (Config, error) {
	var c Config
	path, err := configPath()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(data, &c)
}

func saveConfig(c Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600) // token inside — keep it private
}
