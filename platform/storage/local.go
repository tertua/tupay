package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// localStorage stores files under a directory and serves public ones
// under /uploads/<key>.
type localStorage struct {
	dir string
}

func openLocal(dir string) (*localStorage, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("STORAGE_DIR is required for the local backend")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &localStorage{dir: dir}, nil
}

func (s *localStorage) path(key string) (string, error) {
	// Clean anchors at root, so ".." segments can never escape s.dir.
	clean := filepath.Clean("/" + key)[1:]
	if clean == "" || clean == "." {
		return "", fmt.Errorf("invalid storage key %q", key)
	}
	return filepath.Join(s.dir, filepath.FromSlash(clean)), nil
}

func (s *localStorage) Put(_ context.Context, key string, data io.Reader, _ int64, _ string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	// #nosec G304 -- path is built by (*localStorage).path, which anchors
	// Clean keys under s.dir so ".." segments can never escape it.
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(f, data)
	return err
}

func (s *localStorage) Get(_ context.Context, key string) (rc io.ReadCloser, contentType string, err error) {
	path, err := s.path(key)
	if err != nil {
		return nil, "", err
	}
	// #nosec G304 -- path is built by (*localStorage).path, which anchors
	// Clean keys under s.dir so ".." segments can never escape it.
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	return f, contentTypeOf(path), nil
}

func (s *localStorage) Delete(_ context.Context, key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *localStorage) URLFor(key string, visibility Visibility) string {
	if visibility == Public {
		return "/uploads/" + strings.TrimPrefix(key, "/")
	}
	return key
}

func contentTypeOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		// Never served inline: legacy SVG uploads download instead of
		// executing scripts in the viewer's origin (stored XSS). New SVG
		// uploads are rejected at the controller layer.
		return "application/octet-stream"
	case ".pdf":
		return "application/pdf"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
