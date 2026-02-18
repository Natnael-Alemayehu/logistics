package queue

import (
	"context"
	"encoding/json"
	"time"
)

type Job struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Priority  int             `json:"priority"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
	Attempts  int             `json:"attempts"`
	MaxRetry  int             `json:"max_retry"`
	Error     string          `json:"error,omitempty"`
}

type JobHandler func(ctx context.Context, job *Job) error

type Queue interface {
	Enqueue(ctx context.Context, job *Job) error
	Dequeue(ctx context.Context, jobTypes ...string) (*Job, error)
	Ack(ctx context.Context, jobID string) error
	Nack(ctx context.Context, jobID string, err error) error
	Close() error
}

const (
	JobTypePhotoCompress = "photo_compress"
	JobTypeSMSSend       = "sms_send"
	JobTypeSMSBatch      = "sms_batch"
	JobTypeReportGen     = "report_generate"
	JobTypeDataArchive   = "data_archive"
)

const (
	PriorityLow    = 1
	PriorityNormal = 5
	PriorityHigh   = 10
)
