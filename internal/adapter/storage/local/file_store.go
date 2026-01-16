package local

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type FileStore struct {
	BaseDir string
}

func NewFileStore() (*FileStore, error) {
	baseDir := viper.GetString("STORAGE_DIR")
	if baseDir == "" {
		baseDir = filepath.Join("data", "contracts")
	}

	// Ensure directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &FileStore{BaseDir: baseDir}, nil
}

func (s *FileStore) Save(filename string, content []byte) (string, error) {
	fullPath := filepath.Join(s.BaseDir, filename)
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filename, err)
	}
	return fullPath, nil
}

func (s *FileStore) Get(filename string) ([]byte, error) {
	fullPath := filepath.Join(s.BaseDir, filename)
	return os.ReadFile(fullPath)
}

func (s *FileStore) Exists(filename string) bool {
	fullPath := filepath.Join(s.BaseDir, filename)
	_, err := os.Stat(fullPath)
	return !os.IsNotExist(err)
}
