package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStorage struct {
	uploads map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		uploads: make(map[string][]byte),
	}
}

func (m *mockStorage) Upload(ctx context.Context, path string, reader io.Reader, contentType string) (string, error) {
	data, _ := io.ReadAll(reader)
	m.uploads[path] = data
	return path, nil
}

func (m *mockStorage) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	data, ok := m.uploads[path]
	if !ok {
		return nil, ErrFileNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mockStorage) Delete(ctx context.Context, path string) error {
	delete(m.uploads, path)
	return nil
}

func (m *mockStorage) GetURL(ctx context.Context, path string) (string, error) {
	return "http://localhost/" + path, nil
}

func createTestImage() string {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func createLargeTestImage() string {
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for y := 0; y < 600; y++ {
		for x := 0; x < 800; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestPODStorage_UploadSignature(t *testing.T) {
	mock := newMockStorage()
	pod := NewPODStorage(mock)
	ctx := context.Background()

	t.Run("uploads PNG signature successfully", func(t *testing.T) {
		sigData := createTestImage()
		path, err := pod.UploadSignature(ctx, "shipment-123", sigData)
		require.NoError(t, err)
		assert.Contains(t, path, "pod/shipment-123/signature_")
		assert.Contains(t, path, ".png")
	})

	t.Run("uploads SVG signature successfully", func(t *testing.T) {
		svgData := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte("<svg></svg>"))
		path, err := pod.UploadSignature(ctx, "shipment-456", svgData)
		require.NoError(t, err)
		assert.Contains(t, path, "pod/shipment-456/signature_")
		assert.Contains(t, path, ".svg")
	})

	t.Run("uploads raw SVG signature", func(t *testing.T) {
		svgData := "data:image/svg+xml,<svg></svg>"
		path, err := pod.UploadSignature(ctx, "shipment-789", svgData)
		require.NoError(t, err)
		assert.Contains(t, path, ".svg")
	})

	t.Run("rejects empty signature", func(t *testing.T) {
		_, err := pod.UploadSignature(ctx, "shipment-123", "")
		require.Error(t, err)
		assert.Equal(t, ErrInvalidSignature, err)
	})

	t.Run("handles invalid base64", func(t *testing.T) {
		_, err := pod.UploadSignature(ctx, "shipment-123", "not-valid-base64!!!")
		require.Error(t, err)
	})
}

func TestPODStorage_UploadPhoto(t *testing.T) {
	mock := newMockStorage()
	pod := NewPODStorage(mock)
	ctx := context.Background()

	t.Run("uploads photo successfully", func(t *testing.T) {
		photoData := createTestImage()
		path, err := pod.UploadPhoto(ctx, "shipment-123", photoData, 0)
		require.NoError(t, err)
		assert.Contains(t, path, "pod/shipment-123/photo_1_")
		assert.Contains(t, path, ".jpg")
	})

	t.Run("uploads multiple photos", func(t *testing.T) {
		photoData := createTestImage()
		path1, err := pod.UploadPhoto(ctx, "shipment-123", photoData, 0)
		require.NoError(t, err)
		assert.Contains(t, path1, "photo_1_")

		path2, err := pod.UploadPhoto(ctx, "shipment-123", photoData, 1)
		require.NoError(t, err)
		assert.Contains(t, path2, "photo_2_")

		path3, err := pod.UploadPhoto(ctx, "shipment-123", photoData, 2)
		require.NoError(t, err)
		assert.Contains(t, path3, "photo_3_")
	})

	t.Run("rejects invalid photo index negative", func(t *testing.T) {
		photoData := createTestImage()
		_, err := pod.UploadPhoto(ctx, "shipment-123", photoData, -1)
		require.Error(t, err)
		assert.Equal(t, ErrTooManyPhotos, err)
	})

	t.Run("rejects invalid photo index too large", func(t *testing.T) {
		photoData := createTestImage()
		_, err := pod.UploadPhoto(ctx, "shipment-123", photoData, 3)
		require.Error(t, err)
		assert.Equal(t, ErrTooManyPhotos, err)
	})

	t.Run("rejects empty photo data", func(t *testing.T) {
		_, err := pod.UploadPhoto(ctx, "shipment-123", "", 0)
		require.Error(t, err)
		assert.Equal(t, ErrInvalidImageData, err)
	})

	t.Run("resizes large photo", func(t *testing.T) {
		largePhoto := createLargeTestImage()
		path, err := pod.UploadPhoto(ctx, "shipment-123", largePhoto, 0)
		require.NoError(t, err)
		assert.NotEmpty(t, path)
	})
}

func TestPODStorage_UploadMultiplePhotos(t *testing.T) {
	mock := newMockStorage()
	pod := NewPODStorage(mock)
	ctx := context.Background()

	t.Run("uploads multiple photos", func(t *testing.T) {
		photos := []string{
			createTestImage(),
			createTestImage(),
			createTestImage(),
		}
		urls, err := pod.UploadMultiplePhotos(ctx, "shipment-123", photos)
		require.NoError(t, err)
		assert.Len(t, urls, 3)
	})

	t.Run("skips empty photos", func(t *testing.T) {
		photos := []string{
			createTestImage(),
			"",
			createTestImage(),
		}
		urls, err := pod.UploadMultiplePhotos(ctx, "shipment-123", photos)
		require.NoError(t, err)
		assert.Len(t, urls, 2)
	})

	t.Run("rejects too many photos", func(t *testing.T) {
		photos := []string{
			createTestImage(),
			createTestImage(),
			createTestImage(),
			createTestImage(),
		}
		_, err := pod.UploadMultiplePhotos(ctx, "shipment-123", photos)
		require.Error(t, err)
		assert.Equal(t, ErrTooManyPhotos, err)
	})

	t.Run("handles empty list", func(t *testing.T) {
		urls, err := pod.UploadMultiplePhotos(ctx, "shipment-123", []string{})
		require.NoError(t, err)
		assert.Empty(t, urls)
	})
}

func TestPODStorage_GetPODURLs(t *testing.T) {
	mock := newMockStorage()
	pod := NewPODStorage(mock)
	ctx := context.Background()

	t.Run("returns POD URLs", func(t *testing.T) {
		urls, err := pod.GetPODURLs(ctx, "shipment-123")
		require.NoError(t, err)
		require.NotNil(t, urls)
	})
}

func TestPODStorage_DeletePODFiles(t *testing.T) {
	mock := newMockStorage()
	pod := NewPODStorage(mock)
	ctx := context.Background()

	t.Run("deletes POD files", func(t *testing.T) {
		err := pod.DeletePODFiles(ctx, "shipment-123")
		require.NoError(t, err)
	})
}

func TestPODStorage_DecodeBase64Image(t *testing.T) {
	pod := &PODStorage{}

	t.Run("decodes with data URI prefix", func(t *testing.T) {
		imgData := createTestImage()
		img, err := pod.decodeBase64Image(imgData)
		require.NoError(t, err)
		require.NotNil(t, img)
	})

	t.Run("decodes without data URI prefix", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 10, 10))
		var buf bytes.Buffer
		png.Encode(&buf, img)
		rawBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())

		decoded, err := pod.decodeBase64Image(rawBase64)
		require.NoError(t, err)
		require.NotNil(t, decoded)
	})

	t.Run("handles raw encoding", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 10, 10))
		var buf bytes.Buffer
		png.Encode(&buf, img)
		rawBase64 := base64.RawStdEncoding.EncodeToString(buf.Bytes())

		decoded, err := pod.decodeBase64Image(rawBase64)
		require.NoError(t, err)
		require.NotNil(t, decoded)
	})

	t.Run("returns error for invalid base64", func(t *testing.T) {
		_, err := pod.decodeBase64Image("!!!invalid!!!")
		require.Error(t, err)
	})

	t.Run("returns error for invalid image data", func(t *testing.T) {
		validBase64 := base64.StdEncoding.EncodeToString([]byte("not an image"))
		_, err := pod.decodeBase64Image(validBase64)
		require.Error(t, err)
	})
}

func TestPODStorage_ResizeImage(t *testing.T) {
	pod := &PODStorage{}

	t.Run("does not resize small image", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		resized := pod.resizeImage(img, 800, 600)
		assert.Equal(t, 100, resized.Bounds().Dx())
		assert.Equal(t, 100, resized.Bounds().Dy())
	})

	t.Run("resizes large image proportionally", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 1600, 1200))
		resized := pod.resizeImage(img, 800, 600)
		assert.Equal(t, 800, resized.Bounds().Dx())
		assert.Equal(t, 600, resized.Bounds().Dy())
	})

	t.Run("handles width-only resize", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 1600, 400))
		resized := pod.resizeImage(img, 800, 600)
		assert.Equal(t, 800, resized.Bounds().Dx())
		assert.LessOrEqual(t, resized.Bounds().Dy(), 600)
	})

	t.Run("handles height-only resize", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 400, 1200))
		resized := pod.resizeImage(img, 800, 600)
		assert.LessOrEqual(t, resized.Bounds().Dx(), 800)
		assert.Equal(t, 600, resized.Bounds().Dy())
	})
}

func TestPODStorage_CompressJPEG(t *testing.T) {
	pod := &PODStorage{}

	t.Run("compresses image", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		compressed, err := pod.compressJPEG(img)
		require.NoError(t, err)
		assert.NotEmpty(t, compressed)
	})

	t.Run("reduces quality for large images", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 2000, 2000))
		for y := 0; y < 2000; y++ {
			for x := 0; x < 2000; x++ {
				img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: uint8((x + y) % 256), A: 255})
			}
		}
		compressed, err := pod.compressJPEG(img)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(compressed), TargetPhotoSize*2)
	})
}

func TestPODStorage_CompressPNG(t *testing.T) {
	pod := &PODStorage{}

	t.Run("compresses image as PNG", func(t *testing.T) {
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		compressed, err := pod.compressPNG(img)
		require.NoError(t, err)
		assert.NotEmpty(t, compressed)
	})
}

func TestConstants(t *testing.T) {
	assert.Equal(t, 800, MaxPhotoWidth)
	assert.Equal(t, 600, MaxPhotoHeight)
	assert.Equal(t, 70, JPEGQuality)
	assert.Equal(t, 500*1024, TargetPhotoSize)
	assert.Equal(t, 3, MaxPhotosPerPOD)
	assert.Equal(t, 400, SignatureMaxWidth)
	assert.Equal(t, 200, SignatureMaxHeight)
}

func TestPODErrors(t *testing.T) {
	assert.Equal(t, "exceeded maximum photos per delivery", ErrTooManyPhotos.Error())
	assert.Equal(t, "invalid image data", ErrInvalidImageData.Error())
	assert.Equal(t, "invalid signature data", ErrInvalidSignature.Error())
}

func TestPODURLs(t *testing.T) {
	urls := &PODURLs{
		SignatureURL: "http://localhost/signature.png",
		PhotoURLs:    []string{"http://localhost/photo1.jpg", "http://localhost/photo2.jpg"},
	}

	assert.Equal(t, "http://localhost/signature.png", urls.SignatureURL)
	assert.Len(t, urls.PhotoURLs, 2)
}

func BenchmarkPODStorage_UploadPhoto(b *testing.B) {
	mock := newMockStorage()
	pod := NewPODStorage(mock)
	ctx := context.Background()
	photoData := createTestImage()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pod.UploadPhoto(ctx, "shipment-123", photoData, 0)
	}
}

func BenchmarkPODStorage_ResizeImage(b *testing.B) {
	pod := &PODStorage{}
	img := image.NewRGBA(image.Rect(0, 0, 1600, 1200))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pod.resizeImage(img, 800, 600)
	}
}

func BenchmarkPODStorage_CompressJPEG(b *testing.B) {
	pod := &PODStorage{}
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pod.compressJPEG(img)
	}
}
