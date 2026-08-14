package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type LocalStorage struct {
	basePath string
	baseURL  string
}

func NewLocalStorage(basePath string) (*LocalStorage, error) {
	if basePath == "" {
		basePath = "./uploads"
	}

	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorage{
		basePath: absPath,
		baseURL:  "/uploads",
	}, nil
}

func (l *LocalStorage) Upload(ctx context.Context, path string, reader io.Reader, contentType string) (string, error) {
	if err := l.validatePath(path); err != nil {
		return "", err
	}

	fullPath := filepath.Join(l.basePath, path)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		os.Remove(fullPath)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return path, nil
}

func (l *LocalStorage) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	if err := l.validatePath(path); err != nil {
		return nil, err
	}

	fullPath := filepath.Join(l.basePath, path)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

func (l *LocalStorage) Delete(ctx context.Context, path string) error {
	if err := l.validatePath(path); err != nil {
		return err
	}

	fullPath := filepath.Join(l.basePath, path)

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return ErrFileNotFound
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

func (l *LocalStorage) GetURL(ctx context.Context, path string) (string, error) {
	if err := l.validatePath(path); err != nil {
		return "", err
	}

	return l.baseURL + "/" + strings.TrimPrefix(path, "/"), nil
}

func (l *LocalStorage) validatePath(path string) error {
	if path == "" || strings.Contains(path, "..") {
		return ErrInvalidPath
	}

	cleanPath := filepath.Clean(path)
	if strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") {
		cleanPath = cleanPath[1:]
	}

	fullPath := filepath.Join(l.basePath, cleanPath)
	if !strings.HasPrefix(fullPath, l.basePath) {
		return ErrInvalidPath
	}

	return nil
}

func (l *LocalStorage) GetBasePath() string {
	return l.basePath
}
