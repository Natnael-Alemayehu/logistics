package storage

import (
	"bytes"
	"context"
	"encoding/base64"
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
)

var (
	ErrTooManyPhotos    = errors.New("exceeded maximum photos per delivery")
	ErrInvalidImageData = errors.New("invalid image data")
	ErrInvalidSignature = errors.New("invalid signature data")
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

	var img image.Image
	var err error

	if strings.HasPrefix(signatureData, "data:image/svg+xml") {
		return p.uploadSVGSignature(ctx, shipmentID, signatureData)
	}

	img, err = p.decodeBase64Image(signatureData)
	if err != nil {
		return "", fmt.Errorf("failed to decode signature: %w", err)
	}

	resized := p.resizeImage(img, SignatureMaxWidth, SignatureMaxHeight)

	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return "", fmt.Errorf("failed to encode signature as PNG: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102_150405")
	path := fmt.Sprintf("pod/%s/signature_%s.png", shipmentID, timestamp)

	reader := bytes.NewReader(buf.Bytes())
	if _, err := p.storage.Upload(ctx, path, reader, "image/png"); err != nil {
		return "", fmt.Errorf("failed to upload signature: %w", err)
	}

	return path, nil
}

func (p *PODStorage) uploadSVGSignature(ctx context.Context, shipmentID, signatureData string) (string, error) {
	data := signatureData
	if strings.HasPrefix(data, "data:image/svg+xml;base64,") {
		data = strings.TrimPrefix(data, "data:image/svg+xml;base64,")
	} else if strings.HasPrefix(data, "data:image/svg+xml,") {
		data = strings.TrimPrefix(data, "data:image/svg+xml,")
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		decoded = []byte(data)
	}

	timestamp := time.Now().UTC().Format("20060102_150405")
	path := fmt.Sprintf("pod/%s/signature_%s.svg", shipmentID, timestamp)

	reader := bytes.NewReader(decoded)
	if _, err := p.storage.Upload(ctx, path, reader, "image/svg+xml"); err != nil {
		return "", fmt.Errorf("failed to upload SVG signature: %w", err)
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

	timestamp := time.Now().UTC().Format("20060102_150405")
	path := fmt.Sprintf("pod/%s/photo_%d_%s.jpg", shipmentID, photoIndex+1, timestamp)

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
			return nil, fmt.Errorf("failed to upload photo %d: %w", i+1, err)
		}
		urls = append(urls, url)
	}

	return urls, nil
}

func (p *PODStorage) GetPODURLs(ctx context.Context, shipmentID string) (*PODURLs, error) {
	urls := &PODURLs{}

	signaturePattern := fmt.Sprintf("pod/%s/signature_", shipmentID)

	sigURL, err := p.storage.GetURL(ctx, signaturePattern+"latest.png")
	if err == nil {
		urls.SignatureURL = sigURL
	}

	for i := 0; i < MaxPhotosPerPOD; i++ {
		photoPath := fmt.Sprintf("pod/%s/photo_%d_latest.jpg", shipmentID, i+1)
		photoURL, err := p.storage.GetURL(ctx, photoPath)
		if err == nil {
			urls.PhotoURLs = append(urls.PhotoURLs, photoURL)
		}
	}

	return urls, nil
}

type PODURLs struct {
	SignatureURL string   `json:"signature_url"`
	PhotoURLs    []string `json:"photo_urls"`
}

func (p *PODStorage) DeletePODFiles(ctx context.Context, shipmentID string) error {
	paths := []string{}

	sigPattern := fmt.Sprintf("pod/%s/signature_", shipmentID)
	for _, ext := range []string{".png", ".svg"} {
		paths = append(paths, sigPattern+ext)
	}

	for i := 0; i < MaxPhotosPerPOD; i++ {
		paths = append(paths, fmt.Sprintf("pod/%s/photo_%d.jpg", shipmentID, i+1))
	}

	for _, path := range paths {
		_ = p.storage.Delete(ctx, path)
	}

	return nil
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

	reader := bytes.NewReader(decoded)
	img, _, err := image.Decode(reader)
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
