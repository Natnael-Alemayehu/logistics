package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/natnael-alemayehu/logistics/pkg/storage"
)

// MediaService stores proof-of-delivery images. Files are addressed by a
// storage key recorded on the POD and only ever served through handlers
// that check the viewer may see that shipment.
type MediaService struct {
	*core
	store storage.Storage
	pod   *storage.PODStorage
}

func podStorage(st storage.Storage) *storage.PODStorage {
	if st == nil {
		return nil
	}
	return storage.NewPODStorage(st)
}

// Limits on what a device may upload. Photos arrive already reduced to
// 800×600 by the app; these bounds leave room for older clients.
const (
	maxSignatureBytes = 512 << 10
	maxPhotoBytes     = 4 << 20
)

// ErrMediaInvalid marks upload data that will never be accepted, as opposed
// to a storage failure worth retrying.
var ErrMediaInvalid = &Error{Kind: KindInvalid, Field: "photos", Message: "A photo or the signature could not be read."}

// SavePOD stores a signature and photos given as data URLs, returning their
// keys.
func (m *MediaService) SavePOD(ctx context.Context, shipmentID, signature string, photos []string) (string, []string, error) {
	if m.pod == nil {
		return "", nil, errors.New("media storage is not configured")
	}
	if signature == "" {
		return "", nil, invalid("signature", "The recipient's signature is required.")
	}
	if len(signature) > maxSignatureBytes {
		return "", nil, ErrMediaInvalid
	}
	if len(photos) > storage.MaxPhotosPerPOD {
		return "", nil, invalid("photos", fmt.Sprintf("Attach at most %d photos.", storage.MaxPhotosPerPOD))
	}
	for _, p := range photos {
		if len(p) > maxPhotoBytes {
			return "", nil, ErrMediaInvalid
		}
	}
	sig, err := m.pod.UploadSignature(ctx, shipmentID, signature)
	if err != nil {
		return "", nil, classifyUpload(err)
	}
	keys, err := m.pod.UploadMultiplePhotos(ctx, shipmentID, photos)
	if err != nil {
		m.Delete(context.WithoutCancel(ctx), sig)
		return "", nil, classifyUpload(err)
	}
	return sig, keys, nil
}

// Delete removes stored files whose proof of delivery was refused.
func (m *MediaService) Delete(ctx context.Context, keys ...string) {
	if m.pod != nil {
		m.pod.Delete(ctx, keys...)
	}
}

func classifyUpload(err error) error {
	msg := err.Error()
	if errors.Is(err, storage.ErrInvalidImageData) || errors.Is(err, storage.ErrInvalidSignature) ||
		strings.Contains(msg, "decode") {
		return ErrMediaInvalid
	}
	return err
}

// Open streams a stored file and reports its content type.
func (m *MediaService) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if m.store == nil {
		return nil, "", notFound("File not found.")
	}
	rc, err := m.store.Download(ctx, key)
	if errors.Is(err, storage.ErrFileNotFound) {
		return nil, "", notFound("File not found.")
	}
	if err != nil {
		return nil, "", err
	}
	ct := "application/octet-stream"
	switch strings.ToLower(path.Ext(key)) {
	case ".jpg", ".jpeg":
		ct = "image/jpeg"
	case ".png":
		ct = "image/png"
	case ".svg":
		ct = "image/svg+xml"
	}
	return rc, ct, nil
}
