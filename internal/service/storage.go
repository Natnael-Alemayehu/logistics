package service

import (
	"context"
	"fmt"

	"github.com/natnael-alemayehu/logistics/internal/config"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/pkg/storage"
)

type PODStorageService struct {
	queries  *db.Queries
	podStore *storage.PODStorage
}

func NewPODStorageService(queries *db.Queries, cfg *config.Config) (*PODStorageService, error) {
	storeCfg := &storage.Config{
		Type:        cfg.StorageType,
		LocalPath:   cfg.StoragePath,
		S3Bucket:    cfg.S3Bucket,
		S3Region:    cfg.S3Region,
		S3Endpoint:  cfg.S3Endpoint,
		S3AccessKey: cfg.S3AccessKey,
		S3SecretKey: cfg.S3SecretKey,
		S3UseSSL:    cfg.S3UseSSL,
	}

	store, err := storage.New(storeCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	return &PODStorageService{
		queries:  queries,
		podStore: storage.NewPODStorage(store),
	}, nil
}

func NewPODStorageServiceWithStorage(queries *db.Queries, store storage.Storage) *PODStorageService {
	return &PODStorageService{
		queries:  queries,
		podStore: storage.NewPODStorage(store),
	}
}

type PODUploadInput struct {
	ShipmentID      string
	DriverID        string
	TenantID        string
	RecipientName   string
	RecipientPhone  string
	SignatureData   string
	Photos          []string
	DeliveryAddress string
	DeliveryLat     float64
	DeliveryLng     float64
	DeliveryNotes   string
	RecordedAt      string
}

type PODUploadResult struct {
	ID           string
	SignatureURL string
	PhotoURLs    []string
}

func (s *PODStorageService) UploadPOD(ctx context.Context, input PODUploadInput) (*PODUploadResult, error) {
	var signatureURL string
	var photoURLs []string
	var err error

	if input.SignatureData != "" {
		signatureURL, err = s.podStore.UploadSignature(ctx, input.ShipmentID, input.SignatureData)
		if err != nil {
			return nil, fmt.Errorf("failed to upload signature: %w", err)
		}
	}

	if len(input.Photos) > 0 {
		photoURLs, err = s.podStore.UploadMultiplePhotos(ctx, input.ShipmentID, input.Photos)
		if err != nil {
			return nil, fmt.Errorf("failed to upload photos: %w", err)
		}
	}

	return &PODUploadResult{
		SignatureURL: signatureURL,
		PhotoURLs:    photoURLs,
	}, nil
}

func (s *PODStorageService) ProcessPODInput(ctx context.Context, tenantID, driverID string, pod model.PODInput) (*PODUploadResult, error) {
	input := PODUploadInput{
		ShipmentID:      pod.ShipmentID,
		DriverID:        driverID,
		TenantID:        tenantID,
		RecipientName:   pod.RecipientName,
		RecipientPhone:  pod.RecipientPhone,
		SignatureData:   pod.SignatureData,
		Photos:          pod.PhotoURLs,
		DeliveryAddress: pod.DeliveryAddress,
		DeliveryLat:     pod.DeliveryLat,
		DeliveryLng:     pod.DeliveryLng,
		DeliveryNotes:   pod.DeliveryNotes,
	}

	return s.UploadPOD(ctx, input)
}

func (s *PODStorageService) GetPODURLs(ctx context.Context, shipmentID string) (*storage.PODURLs, error) {
	return s.podStore.GetPODURLs(ctx, shipmentID)
}

func (s *PODStorageService) DeletePODFiles(ctx context.Context, shipmentID string) error {
	return s.podStore.DeletePODFiles(ctx, shipmentID)
}

func (s *PODStorageService) UploadSignature(ctx context.Context, shipmentID, signatureData string) (string, error) {
	return s.podStore.UploadSignature(ctx, shipmentID, signatureData)
}

func (s *PODStorageService) UploadPhoto(ctx context.Context, shipmentID, imageData string, photoIndex int) (string, error) {
	return s.podStore.UploadPhoto(ctx, shipmentID, imageData, photoIndex)
}

func (s *PODStorageService) UploadMultiplePhotos(ctx context.Context, shipmentID string, photos []string) ([]string, error) {
	return s.podStore.UploadMultiplePhotos(ctx, shipmentID, photos)
}
