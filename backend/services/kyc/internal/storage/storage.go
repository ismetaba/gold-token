// Package storage provides an abstracted document store for KYC files.
//
// The POC implementation writes to the local filesystem. The interface is
// designed to be swapped for an S3-compatible backend without changing callers.
package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Store is the document storage abstraction.
type Store interface {
	// Save persists data under a generated path and returns the relative path.
	Save(ctx context.Context, userID uuid.UUID, originalFilename string, data []byte) (relativePath string, err error)
}

// LocalStore writes documents to a directory on the local filesystem.
type LocalStore struct {
	Root string // absolute path to storage root
}

// NewLocalStore creates a LocalStore, creating Root if it does not exist.
func NewLocalStore(root string) (*LocalStore, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create storage root %q: %w", root, err)
	}
	return &LocalStore{Root: root}, nil
}

// Save writes data to <root>/<userID>/<timestamp>_<uuid>.<ext> and returns
// the path relative to Root.
func (s *LocalStore) Save(_ context.Context, userID uuid.UUID, originalFilename string, data []byte) (string, error) {
	dir := filepath.Join(s.Root, userID.String())
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create user dir: %w", err)
	}

	ext := filepath.Ext(originalFilename)
	filename := fmt.Sprintf("%d_%s%s", time.Now().UnixMilli(), uuid.Must(uuid.NewV7()).String(), ext)
	absPath := filepath.Join(dir, filename)

	if err := os.WriteFile(absPath, data, 0o640); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	// Return path relative to Root for portability.
	rel, err := filepath.Rel(s.Root, absPath)
	if err != nil {
		return "", fmt.Errorf("rel path: %w", err)
	}
	return rel, nil
}
