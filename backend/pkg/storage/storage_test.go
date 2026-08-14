package storage

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("creates local storage with default path", func(t *testing.T) {
		cfg := &Config{Type: "local"}
		storage, err := New(cfg)
		require.NoError(t, err)
		require.NotNil(t, storage)
		_, ok := storage.(*LocalStorage)
		assert.True(t, ok)
	})

	t.Run("creates local storage with custom path", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := &Config{Type: "local", LocalPath: tmpDir}
		storage, err := New(cfg)
		require.NoError(t, err)
		require.NotNil(t, storage)
	})

	t.Run("creates local storage with empty type", func(t *testing.T) {
		cfg := &Config{Type: ""}
		storage, err := New(cfg)
		require.NoError(t, err)
		require.NotNil(t, storage)
	})

	t.Run("returns error for unknown type", func(t *testing.T) {
		cfg := &Config{Type: "unknown"}
		storage, err := New(cfg)
		require.Error(t, err)
		assert.Equal(t, ErrInvalidConfig, err)
		assert.Nil(t, storage)
	})
}

func TestLocalStorage_NewLocalStorage(t *testing.T) {
	t.Run("creates storage with default path", func(t *testing.T) {
		storage, err := NewLocalStorage("")
		require.NoError(t, err)
		require.NotNil(t, storage)
		assert.Contains(t, storage.GetBasePath(), "uploads")
		_ = storage
	})

	t.Run("creates storage with custom path", func(t *testing.T) {
		tmpDir := t.TempDir()
		storage, err := NewLocalStorage(tmpDir)
		require.NoError(t, err)
		require.NotNil(t, storage)
		assert.Equal(t, tmpDir, storage.GetBasePath())
	})

	t.Run("creates directory if not exists", func(t *testing.T) {
		tmpDir := filepath.Join(t.TempDir(), "subdir", "storage")
		_, err := NewLocalStorage(tmpDir)
		require.NoError(t, err)
		assert.DirExists(t, tmpDir)
	})
}

func TestLocalStorage_Upload(t *testing.T) {
	storage, err := NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("uploads file successfully", func(t *testing.T) {
		content := []byte("test content")
		reader := bytes.NewReader(content)
		path, err := storage.Upload(ctx, "test/file.txt", reader, "text/plain")
		require.NoError(t, err)
		assert.Equal(t, "test/file.txt", path)
	})

	t.Run("uploads to nested directory", func(t *testing.T) {
		content := []byte("nested content")
		reader := bytes.NewReader(content)
		path, err := storage.Upload(ctx, "a/b/c/file.txt", reader, "text/plain")
		require.NoError(t, err)
		assert.Equal(t, "a/b/c/file.txt", path)
	})

	t.Run("rejects empty path", func(t *testing.T) {
		reader := bytes.NewReader([]byte("content"))
		_, err := storage.Upload(ctx, "", reader, "text/plain")
		require.Error(t, err)
		assert.Equal(t, ErrInvalidPath, err)
	})

	t.Run("rejects path traversal", func(t *testing.T) {
		reader := bytes.NewReader([]byte("content"))
		_, err := storage.Upload(ctx, "../etc/passwd", reader, "text/plain")
		require.Error(t, err)
		assert.Equal(t, ErrInvalidPath, err)
	})

	t.Run("rejects path with double dots", func(t *testing.T) {
		reader := bytes.NewReader([]byte("content"))
		_, err := storage.Upload(ctx, "test/../../../etc/passwd", reader, "text/plain")
		require.Error(t, err)
		assert.Equal(t, ErrInvalidPath, err)
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		reader1 := bytes.NewReader([]byte("original"))
		_, err := storage.Upload(ctx, "overwrite.txt", reader1, "text/plain")
		require.NoError(t, err)

		reader2 := bytes.NewReader([]byte("updated"))
		_, err = storage.Upload(ctx, "overwrite.txt", reader2, "text/plain")
		require.NoError(t, err)

		rc, err := storage.Download(ctx, "overwrite.txt")
		require.NoError(t, err)
		defer rc.Close()
		content, _ := io.ReadAll(rc)
		assert.Equal(t, "updated", string(content))
	})
}

func TestLocalStorage_Download(t *testing.T) {
	storage, err := NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("downloads existing file", func(t *testing.T) {
		content := []byte("downloadable content")
		_, err := storage.Upload(ctx, "download.txt", bytes.NewReader(content), "text/plain")
		require.NoError(t, err)

		reader, err := storage.Download(ctx, "download.txt")
		require.NoError(t, err)
		defer reader.Close()

		downloaded, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, content, downloaded)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		_, err := storage.Download(ctx, "nonexistent.txt")
		require.Error(t, err)
		assert.Equal(t, ErrFileNotFound, err)
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		_, err := storage.Download(ctx, "../etc/passwd")
		require.Error(t, err)
		assert.Equal(t, ErrInvalidPath, err)
	})

	t.Run("rejects empty path", func(t *testing.T) {
		_, err := storage.Download(ctx, "")
		require.Error(t, err)
		assert.Equal(t, ErrInvalidPath, err)
	})
}

func TestLocalStorage_Delete(t *testing.T) {
	storage, err := NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("deletes existing file", func(t *testing.T) {
		_, err := storage.Upload(ctx, "delete.txt", bytes.NewReader([]byte("content")), "text/plain")
		require.NoError(t, err)

		err = storage.Delete(ctx, "delete.txt")
		require.NoError(t, err)

		_, err = storage.Download(ctx, "delete.txt")
		assert.Equal(t, ErrFileNotFound, err)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		err := storage.Delete(ctx, "nonexistent.txt")
		require.Error(t, err)
		assert.Equal(t, ErrFileNotFound, err)
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		err := storage.Delete(ctx, "../etc/passwd")
		require.Error(t, err)
		assert.Equal(t, ErrInvalidPath, err)
	})

	t.Run("rejects empty path", func(t *testing.T) {
		err := storage.Delete(ctx, "")
		require.Error(t, err)
		assert.Equal(t, ErrInvalidPath, err)
	})
}

func TestLocalStorage_GetURL(t *testing.T) {
	storage, err := NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("returns URL for valid path", func(t *testing.T) {
		url, err := storage.GetURL(ctx, "test/file.txt")
		require.NoError(t, err)
		assert.Contains(t, url, "/uploads/")
		assert.Contains(t, url, "test/file.txt")
	})

	t.Run("returns URL without leading slash", func(t *testing.T) {
		url, err := storage.GetURL(ctx, "/leading/slash.txt")
		require.NoError(t, err)
		assert.Contains(t, url, "leading/slash.txt")
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		_, err := storage.GetURL(ctx, "../etc/passwd")
		require.Error(t, err)
		assert.Equal(t, ErrInvalidPath, err)
	})

	t.Run("rejects empty path", func(t *testing.T) {
		_, err := storage.GetURL(ctx, "")
		require.Error(t, err)
		assert.Equal(t, ErrInvalidPath, err)
	})
}

func TestLocalStorage_ValidatePath(t *testing.T) {
	storage, err := NewLocalStorage(t.TempDir())
	require.NoError(t, err)

	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{"valid simple", "file.txt", nil},
		{"valid nested", "a/b/c/file.txt", nil},
		{"empty path", "", ErrInvalidPath},
		{"double dot", "..", ErrInvalidPath},
		{"path traversal", "../etc/passwd", ErrInvalidPath},
		{"mixed traversal", "valid/../etc/passwd", ErrInvalidPath},
		{"leading slash", "/absolute/path", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := storage.validatePath(tt.path)
			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLocalStorage_GetBasePath(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewLocalStorage(tmpDir)
	require.NoError(t, err)

	absPath, _ := filepath.Abs(tmpDir)
	assert.Equal(t, absPath, storage.GetBasePath())
}

func TestErrors(t *testing.T) {
	t.Run("error types are defined", func(t *testing.T) {
		assert.Equal(t, "file not found", ErrFileNotFound.Error())
		assert.Equal(t, "invalid file path", ErrInvalidPath.Error())
		assert.Equal(t, "storage operation failed", ErrStorageFailed.Error())
		assert.Equal(t, "invalid storage configuration", ErrInvalidConfig.Error())
	})
}

func TestConfig_Fields(t *testing.T) {
	cfg := &Config{
		Type:        "s3",
		LocalPath:   "/tmp/uploads",
		S3Bucket:    "my-bucket",
		S3Region:    "us-east-1",
		S3Endpoint:  "https://s3.amazonaws.com",
		S3AccessKey: "access-key",
		S3SecretKey: "secret-key",
		S3UseSSL:    true,
	}

	assert.Equal(t, "s3", cfg.Type)
	assert.Equal(t, "my-bucket", cfg.S3Bucket)
	assert.True(t, cfg.S3UseSSL)
}

func TestLocalStorage_LargeFile(t *testing.T) {
	storage, err := NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	largeContent := make([]byte, 10*1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	reader := bytes.NewReader(largeContent)
	path, err := storage.Upload(ctx, "large.bin", reader, "application/octet-stream")
	require.NoError(t, err)

	rc, err := storage.Download(ctx, path)
	require.NoError(t, err)
	defer rc.Close()

	downloaded, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, largeContent, downloaded)
}

func TestLocalStorage_ConcurrentOperations(t *testing.T) {
	storage, err := NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			path := filepath.Join("concurrent", "file"+string(rune('0'+id))+".txt")
			_, err := storage.Upload(ctx, path, bytes.NewReader([]byte("content")), "text/plain")
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkLocalStorage_Upload(b *testing.B) {
	storage, _ := NewLocalStorage(b.TempDir())
	ctx := context.Background()
	content := []byte("benchmark content")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(content)
		_, _ = storage.Upload(ctx, "bench.txt", reader, "text/plain")
	}
}

func BenchmarkLocalStorage_Download(b *testing.B) {
	storage, _ := NewLocalStorage(b.TempDir())
	ctx := context.Background()
	storage.Upload(ctx, "bench.txt", bytes.NewReader([]byte("benchmark content")), "text/plain")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rc, _ := storage.Download(ctx, "bench.txt")
		if rc != nil {
			rc.Close()
		}
	}
}
