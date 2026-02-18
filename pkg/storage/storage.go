package storage

import (
	"context"
	"errors"
	"io"
)

var (
	ErrFileNotFound  = errors.New("file not found")
	ErrInvalidPath   = errors.New("invalid file path")
	ErrStorageFailed = errors.New("storage operation failed")
	ErrInvalidConfig = errors.New("invalid storage configuration")
)

type Storage interface {
	Upload(ctx context.Context, path string, reader io.Reader, contentType string) (string, error)
	Download(ctx context.Context, path string) (io.ReadCloser, error)
	Delete(ctx context.Context, path string) error
	GetURL(ctx context.Context, path string) (string, error)
}

type Config struct {
	Type string

	LocalPath string

	S3Bucket    string
	S3Region    string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool
}

func New(cfg *Config) (Storage, error) {
	switch cfg.Type {
	case "local", "":
		return NewLocalStorage(cfg.LocalPath)
	case "s3":
		return NewS3Storage(cfg.S3Bucket, cfg.S3Region, cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3UseSSL)
	default:
		return nil, ErrInvalidConfig
	}
}
