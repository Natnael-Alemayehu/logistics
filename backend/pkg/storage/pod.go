package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
	"time"
)

const (
	MaxPhotoWidth      = 800
	MaxPhotoHeight     = 600
	JPEGQuality        = 70
	TargetPhotoSize    = 500 * 1024
	MaxPhotosPerPOD    = 3
	SignatureMaxWidth  = 400
	SignatureMaxHeight = 200
	// MaxImageSide is the most pixels a side an uploaded image may have.
	// Decoding allocates every pixel, so a small file claiming a huge size
	// (a blank 8000×8000 PNG is a few kilobytes) would take gigabytes. A
	// phone camera's photo is about 4000 a side.
	MaxImageSide = 4096
)

var (
	ErrTooManyPhotos    = errors.New("exceeded maximum photos per delivery")
	ErrInvalidImageData = errors.New("invalid image data")
	ErrInvalidSignature = errors.New("invalid signature data")
	ErrImageTooLarge    = errors.New("image too large")
)

type PODStorage struct {
	storage Storage
}

func NewPODStorage(storage Storage) *PODStorage {
	return &PODStorage{storage: storage}
}

func (p *PODStorage) UploadSignature(ctx context.Context, shipmentID, signatureData string) (string, error) {
	if signatureData == "" {
		return "", ErrInvalidSignature
	}

	// An SVG can carry scripts and links, and would be served from the
	// dashboard's own origin: signatures are images drawn on the phone, sent
	// as PNG.
	if strings.HasPrefix(signatureData, "data:image/svg+xml") {
		return "", ErrInvalidSignature
	}

	img, err := p.decodeBase64Image(signatureData)
	if err != nil {
		return "", fmt.Errorf("failed to decode signature: %w", err)
	}

	resized := p.resizeImage(img, SignatureMaxWidth, SignatureMaxHeight)

	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return "", fmt.Errorf("failed to encode signature as PNG: %w", err)
	}

	path := podPath(shipmentID, "signature", ".png")

	reader := bytes.NewReader(buf.Bytes())
	if _, err := p.storage.Upload(ctx, path, reader, "image/png"); err != nil {
		return "", fmt.Errorf("failed to upload signature: %w", err)
	}

	return path, nil
}

func (p *PODStorage) UploadPhoto(ctx context.Context, shipmentID, imageData string, photoIndex int) (string, error) {
	if photoIndex < 0 || photoIndex >= MaxPhotosPerPOD {
		return "", ErrTooManyPhotos
	}

	if imageData == "" {
		return "", ErrInvalidImageData
	}

	img, err := p.decodeBase64Image(imageData)
	if err != nil {
		return "", fmt.Errorf("failed to decode photo: %w", err)
	}

	resized := p.resizeImage(img, MaxPhotoWidth, MaxPhotoHeight)

	compressed, err := p.compressJPEG(resized)
	if err != nil {
		return "", fmt.Errorf("failed to compress photo: %w", err)
	}

	path := podPath(shipmentID, fmt.Sprintf("photo_%d", photoIndex+1), ".jpg")

	reader := bytes.NewReader(compressed)
	if _, err := p.storage.Upload(ctx, path, reader, "image/jpeg"); err != nil {
		return "", fmt.Errorf("failed to upload photo: %w", err)
	}

	return path, nil
}

func (p *PODStorage) UploadMultiplePhotos(ctx context.Context, shipmentID string, photos []string) ([]string, error) {
	if len(photos) > MaxPhotosPerPOD {
		return nil, ErrTooManyPhotos
	}

	urls := make([]string, 0, len(photos))
	for i, photo := range photos {
		if photo == "" {
			continue
		}
		url, err := p.UploadPhoto(ctx, shipmentID, photo, i)
		if err != nil {
			p.Delete(context.WithoutCancel(ctx), urls...)
			return nil, fmt.Errorf("failed to upload photo %d: %w", i+1, err)
		}
		urls = append(urls, url)
	}

	return urls, nil
}

// Delete removes stored files, ignoring any already gone.
func (p *PODStorage) Delete(ctx context.Context, paths ...string) {
	for _, path := range paths {
		if path != "" {
			_ = p.storage.Delete(ctx, path)
		}
	}
}

// podPath names a new file for a shipment's proof of delivery. A shipment
// can have more than one proof (one held for review beside the accepted
// one), so names must never collide, even within a second.
func podPath(shipmentID, name, ext string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("pod/%s/%s_%s_%s%s", shipmentID, name,
		time.Now().UTC().Format("20060102_150405"), hex.EncodeToString(b), ext)
}

func (p *PODStorage) decodeBase64Image(data string) (image.Image, error) {
	if strings.HasPrefix(data, "data:image/") {
		commaIdx := strings.Index(data, ",")
		if commaIdx != -1 {
			data = data[commaIdx+1:]
		}
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64: %w", err)
		}
	}

	// The size first, from the header alone.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(decoded))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	if cfg.Width > MaxImageSide || cfg.Height > MaxImageSide {
		return nil, ErrImageTooLarge
	}
	img, _, err := image.Decode(bytes.NewReader(decoded))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	return img, nil
}

func (p *PODStorage) resizeImage(img image.Image, maxWidth, maxHeight int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width <= maxWidth && height <= maxHeight {
		return img
	}

	ratio := float64(maxWidth) / float64(width)
	if float64(height)*ratio > float64(maxHeight) {
		ratio = float64(maxHeight) / float64(height)
	}

	newWidth := int(float64(width) * ratio)
	newHeight := int(float64(height) * ratio)

	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := int(float64(x) / ratio)
			srcY := int(float64(y) / ratio)
			dst.Set(x, y, img.At(srcX+bounds.Min.X, srcY+bounds.Min.Y))
		}
	}

	return dst
}

func (p *PODStorage) compressJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer

	quality := JPEGQuality
	for quality >= 10 {
		buf.Reset()
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, fmt.Errorf("failed to encode JPEG: %w", err)
		}

		if buf.Len() <= TargetPhotoSize {
			break
		}

		quality -= 10
	}

	return buf.Bytes(), nil
}

func (p *PODStorage) compressPNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode PNG: %w", err)
	}
	return buf.Bytes(), nil
}
