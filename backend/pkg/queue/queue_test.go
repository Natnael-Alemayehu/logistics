package queue

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJob_Struct(t *testing.T) {
	job := &Job{
		ID:        "test-id",
		Type:      JobTypeSMSSend,
		Priority:  PriorityHigh,
		Payload:   json.RawMessage(`{"to":"+251912345678","message":"test"}`),
		CreatedAt: time.Now(),
		Attempts:  0,
		MaxRetry:  3,
		Error:     "",
	}

	assert.Equal(t, "test-id", job.ID)
	assert.Equal(t, JobTypeSMSSend, job.Type)
	assert.Equal(t, PriorityHigh, job.Priority)
	assert.NotEmpty(t, job.Payload)
	assert.Zero(t, job.Attempts)
	assert.Equal(t, 3, job.MaxRetry)
}

func TestJob_MarshalJSON(t *testing.T) {
	job := &Job{
		ID:        "test-id",
		Type:      JobTypeSMSSend,
		Priority:  PriorityNormal,
		Payload:   json.RawMessage(`{"key":"value"}`),
		CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Attempts:  1,
		MaxRetry:  3,
	}

	data, err := json.Marshal(job)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"test-id"`)
	assert.Contains(t, string(data), `"type":"sms_send"`)
	assert.Contains(t, string(data), `"priority":5`)
}

func TestJob_UnmarshalJSON(t *testing.T) {
	jsonData := `{"id":"test-id","type":"sms_send","priority":5,"payload":{"key":"value"},"created_at":"2024-01-01T00:00:00Z","attempts":1,"max_retry":3}`

	var job Job
	err := json.Unmarshal([]byte(jsonData), &job)
	require.NoError(t, err)
	assert.Equal(t, "test-id", job.ID)
	assert.Equal(t, JobTypeSMSSend, job.Type)
	assert.Equal(t, PriorityNormal, job.Priority)
	assert.Equal(t, 1, job.Attempts)
	assert.Equal(t, 3, job.MaxRetry)
}

func TestJobType_Constants(t *testing.T) {
	assert.Equal(t, "photo_compress", JobTypePhotoCompress)
	assert.Equal(t, "sms_send", JobTypeSMSSend)
	assert.Equal(t, "sms_batch", JobTypeSMSBatch)
	assert.Equal(t, "report_generate", JobTypeReportGen)
	assert.Equal(t, "data_archive", JobTypeDataArchive)
}

func TestPriority_Constants(t *testing.T) {
	assert.Equal(t, 1, PriorityLow)
	assert.Equal(t, 5, PriorityNormal)
	assert.Equal(t, 10, PriorityHigh)
}

func TestJobHandler_Type(t *testing.T) {
	var handler JobHandler = func(ctx context.Context, job *Job) error {
		return nil
	}

	assert.NotNil(t, handler)
}

func TestQueue_Interface(t *testing.T) {
	var _ Queue = (*mockQueue)(nil)
}

type mockQueue struct {
	jobs   []*Job
	closed bool
}

func newMockQueue() *mockQueue {
	return &mockQueue{
		jobs: make([]*Job, 0),
	}
}

func (m *mockQueue) Enqueue(ctx context.Context, job *Job) error {
	m.jobs = append(m.jobs, job)
	return nil
}

func (m *mockQueue) Dequeue(ctx context.Context, jobTypes ...string) (*Job, error) {
	if len(m.jobs) == 0 {
		return nil, nil
	}
	job := m.jobs[0]
	m.jobs = m.jobs[1:]
	return job, nil
}

func (m *mockQueue) Ack(ctx context.Context, jobID string) error {
	return nil
}

func (m *mockQueue) Nack(ctx context.Context, jobID string, err error) error {
	return nil
}

func (m *mockQueue) Close() error {
	m.closed = true
	return nil
}

func TestMockQueue_Enqueue(t *testing.T) {
	q := newMockQueue()
	job := &Job{Type: JobTypeSMSSend, Priority: PriorityNormal}

	err := q.Enqueue(context.Background(), job)
	require.NoError(t, err)
	assert.Len(t, q.jobs, 1)
}

func TestMockQueue_Dequeue(t *testing.T) {
	q := newMockQueue()
	_ = q.Enqueue(context.Background(), &Job{ID: "1", Type: JobTypeSMSSend})

	job, err := q.Dequeue(context.Background(), JobTypeSMSSend)
	require.NoError(t, err)
	assert.Equal(t, "1", job.ID)
}

func TestMockQueue_DequeueEmpty(t *testing.T) {
	q := newMockQueue()

	job, err := q.Dequeue(context.Background(), JobTypeSMSSend)
	require.NoError(t, err)
	assert.Nil(t, job)
}

func TestMockQueue_Ack(t *testing.T) {
	q := newMockQueue()

	err := q.Ack(context.Background(), "job-id")
	require.NoError(t, err)
}

func TestMockQueue_Nack(t *testing.T) {
	q := newMockQueue()

	err := q.Nack(context.Background(), "job-id", assert.AnError)
	require.NoError(t, err)
}

func TestMockQueue_Close(t *testing.T) {
	q := newMockQueue()

	err := q.Close()
	require.NoError(t, err)
	assert.True(t, q.closed)
}

func TestJob_PayloadExtraction(t *testing.T) {
	payload := map[string]string{
		"to":      "+251912345678",
		"message": "Hello",
	}
	payloadBytes, _ := json.Marshal(payload)

	job := &Job{
		ID:      "test",
		Type:    JobTypeSMSSend,
		Payload: payloadBytes,
	}

	var extracted map[string]string
	err := json.Unmarshal(job.Payload, &extracted)
	require.NoError(t, err)
	assert.Equal(t, "+251912345678", extracted["to"])
	assert.Equal(t, "Hello", extracted["message"])
}

func TestJob_EmptyPayload(t *testing.T) {
	job := &Job{
		ID:      "test",
		Type:    JobTypeSMSSend,
		Payload: nil,
	}

	assert.Nil(t, job.Payload)
}

func TestJob_ErrorField(t *testing.T) {
	job := &Job{
		ID:    "test",
		Type:  JobTypeSMSSend,
		Error: "connection timeout",
	}

	assert.Equal(t, "connection timeout", job.Error)
}

func TestJob_AttemptsTracking(t *testing.T) {
	job := &Job{
		ID:       "test",
		Type:     JobTypeSMSSend,
		Attempts: 0,
		MaxRetry: 3,
	}

	for i := 0; i < job.MaxRetry; i++ {
		job.Attempts++
	}

	assert.Equal(t, 3, job.Attempts)
	assert.Equal(t, job.MaxRetry, job.Attempts)
}

func TestJob_PriorityOrdering(t *testing.T) {
	jobs := []*Job{
		{ID: "low", Priority: PriorityLow},
		{ID: "high", Priority: PriorityHigh},
		{ID: "normal", Priority: PriorityNormal},
	}

	for _, job := range jobs {
		switch job.ID {
		case "low":
			assert.Equal(t, PriorityLow, job.Priority)
		case "high":
			assert.Equal(t, PriorityHigh, job.Priority)
		case "normal":
			assert.Equal(t, PriorityNormal, job.Priority)
		}
	}
}

func TestJob_CreatedAt(t *testing.T) {
	now := time.Now()
	job := &Job{
		ID:        "test",
		Type:      JobTypeSMSSend,
		CreatedAt: now,
	}

	assert.Equal(t, now, job.CreatedAt)
}

func TestJob_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	job := &Job{Type: JobTypeSMSSend}
	assert.NotNil(t, job)
	assert.Error(t, ctx.Err())
}

func BenchmarkJob_MarshalJSON(b *testing.B) {
	job := &Job{
		ID:        "test-id",
		Type:      JobTypeSMSSend,
		Priority:  PriorityNormal,
		Payload:   json.RawMessage(`{"key":"value"}`),
		CreatedAt: time.Now(),
		Attempts:  0,
		MaxRetry:  3,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(job)
	}
}

func BenchmarkJob_UnmarshalJSON(b *testing.B) {
	data := []byte(`{"id":"test-id","type":"sms_send","priority":5,"payload":{"key":"value"},"created_at":"2024-01-01T00:00:00Z","attempts":0,"max_retry":3}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var job Job
		_ = json.Unmarshal(data, &job)
	}
}
