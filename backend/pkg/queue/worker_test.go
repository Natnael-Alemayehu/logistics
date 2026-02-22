package queue

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/natnael-alemayehu/logistics/pkg/redis"
	"github.com/rs/zerolog"
)

func setupTestWorker(t *testing.T) (*RedisQueue, *Worker, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client, err := redis.NewClient(redis.Config{
		Addr: mr.Addr(),
	})
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}

	queue := NewRedisQueue(client, "test")
	logger := zerolog.Nop()
	worker := NewWorker(queue, 2, logger)

	return queue, worker, mr
}

func TestWorkerProcessesJobs(t *testing.T) {
	queue, worker, mr := setupTestWorker(t)
	defer mr.Close()

	var processedJobs []*Job
	var mu sync.Mutex

	worker.RegisterHandler(JobTypeSMSSend, func(ctx context.Context, job *Job) error {
		mu.Lock()
		defer mu.Unlock()
		processedJobs = append(processedJobs, job)
		return nil
	})

	payload, _ := json.Marshal(map[string]string{"message": "test"})
	job := &Job{
		Type:     JobTypeSMSSend,
		Priority: PriorityNormal,
		Payload:  payload,
	}

	if err := queue.Enqueue(context.Background(), job); err != nil {
		t.Fatalf("failed to enqueue job: %v", err)
	}

	worker.Start()
	defer worker.Stop()

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if len(processedJobs) != 1 {
		t.Errorf("expected 1 processed job, got %d", len(processedJobs))
	}
	mu.Unlock()
}

func TestWorkerRetriesFailedJobs(t *testing.T) {
	queue, worker, mr := setupTestWorker(t)
	defer mr.Close()

	var attempts int
	var mu sync.Mutex

	worker.RegisterHandler(JobTypeSMSSend, func(ctx context.Context, job *Job) error {
		mu.Lock()
		attempts++
		mu.Unlock()
		return errors.New("intentional failure")
	})

	payload, _ := json.Marshal(map[string]string{"message": "test"})
	job := &Job{
		Type:     JobTypeSMSSend,
		Priority: PriorityNormal,
		Payload:  payload,
		MaxRetry: 2,
	}

	if err := queue.Enqueue(context.Background(), job); err != nil {
		t.Fatalf("failed to enqueue job: %v", err)
	}

	worker.Start()
	defer worker.Stop()

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	if attempts < 1 {
		t.Errorf("expected at least 1 attempt, got %d", attempts)
	}
	mu.Unlock()
}

func TestWorkerStopsGracefully(t *testing.T) {
	queue, worker, mr := setupTestWorker(t)
	defer mr.Close()

	worker.RegisterHandler(JobTypeSMSSend, func(ctx context.Context, job *Job) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	worker.Start()

	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	worker.Stop()
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("worker took too long to stop: %v", elapsed)
	}

	_ = queue
}

func TestWorkerNoHandler(t *testing.T) {
	queue, worker, mr := setupTestWorker(t)
	defer mr.Close()

	payload, _ := json.Marshal(map[string]string{"message": "test"})
	job := &Job{
		Type:     JobTypeSMSSend,
		Priority: PriorityNormal,
		Payload:  payload,
		MaxRetry: 1,
	}

	if err := queue.Enqueue(context.Background(), job); err != nil {
		t.Fatalf("failed to enqueue job: %v", err)
	}

	worker.Start()
	defer worker.Stop()

	time.Sleep(200 * time.Millisecond)

	dlqKey := "test:dlq:" + JobTypeSMSSend
	if !mr.Exists(dlqKey) {
		t.Error("job should be in DLQ when no handler is registered")
	}
}
