// Package storage persists binary blobs (screenshots) behind a small interface.
// v1 ships a filesystem backend; an S3 backend can implement the same interface
// later (selected by HIJAU_STORAGE).
package storage

import (
	"os"
	"path/filepath"
)

type Store interface {
	Put(key string, data []byte) error
	Get(key string) ([]byte, error)
	Delete(key string) error
}

// FS stores blobs as files under a base directory.
type FS struct {
	dir string
}

func NewFS(dir string) *FS {
	if dir == "" {
		dir = "./data/screenshots"
	}
	return &FS{dir: dir}
}

// safe joins key under the base dir, neutralising any path-traversal in key.
func (f *FS) safe(key string) string {
	return filepath.Join(f.dir, filepath.Clean("/"+key))
}

func (f *FS) Put(key string, data []byte) error {
	p := f.safe(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func (f *FS) Get(key string) ([]byte, error) {
	return os.ReadFile(f.safe(key))
}

func (f *FS) Delete(key string) error {
	return os.Remove(f.safe(key))
}
