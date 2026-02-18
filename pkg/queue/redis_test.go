package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/natnael-alemayehu/logistics/pkg/redis"
)

func setupTestQueue(t *testing.T) (*RedisQueue, *miniredis.Miniredis) {
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

	return queue, mr
}

func TestEnqueue(t *testing.T) {
	queue, mr := setupTestQueue(t)
	defer mr.Close()

	payload, _ := json.Marshal(map[string]string{"foo": "bar"})
	job := &Job{
		Type:     JobTypeSMSSend,
		Priority: PriorityNormal,
		Payload:  payload,
	}

	err := queue.Enqueue(context.Background(), job)
	if err != nil {
		t.Fatalf("failed to enqueue job: %v", err)
	}

	if job.ID == "" {
		t.Error("job ID should be set after enqueue")
	}

	if job.CreatedAt.IsZero() {
		t.Error("job CreatedAt should be set after enqueue")
	}

	queueKey := "test:queue:" + JobTypeSMSSend
	if !mr.Exists(queueKey) {
		t.Error("queue key should exist in redis")
	}

	jobKey := "test:job:" + job.ID
	if !mr.Exists(jobKey) {
		t.Error("job key should exist in redis")
	}
}

func TestDequeue(t *testing.T) {
	queue, mr := setupTestQueue(t)
	defer mr.Close()

	payload, _ := json.Marshal(map[string]string{"message": "test"})

	highJob := &Job{
		Type:     JobTypeSMSSend,
		Priority: PriorityHigh,
		Payload:  payload,
	}
	lowJob := &Job{
		Type:     JobTypeSMSSend,
		Priority: PriorityLow,
		Payload:  payload,
	}

	if err := queue.Enqueue(context.Background(), lowJob); err != nil {
		t.Fatalf("failed to enqueue low priority job: %v", err)
	}
	if err := queue.Enqueue(context.Background(), highJob); err != nil {
		t.Fatalf("failed to enqueue high priority job: %v", err)
	}

	job, err := queue.Dequeue(context.Background(), JobTypeSMSSend)
	if err != nil {
		t.Fatalf("failed to dequeue job: %v", err)
	}

	if job == nil {
		t.Fatal("expected job, got nil")
	}

	if job.ID != highJob.ID {
		t.Errorf("expected high priority job %s, got %s", highJob.ID, job.ID)
	}
}

func TestDequeueNoJobs(t *testing.T) {
	queue, mr := setupTestQueue(t)
	defer mr.Close()

	job, err := queue.Dequeue(context.Background(), JobTypeSMSSend)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job != nil {
		t.Error("expected nil job when queue is empty")
	}
}

func TestAck(t *testing.T) {
	queue, mr := setupTestQueue(t)
	defer mr.Close()

	payload, _ := json.Marshal(map[string]string{"foo": "bar"})
	job := &Job{
		Type:     JobTypeSMSSend,
		Priority: PriorityNormal,
		Payload:  payload,
	}

	if err := queue.Enqueue(context.Background(), job); err != nil {
		t.Fatalf("failed to enqueue job: %v", err)
	}

	if _, err := queue.Dequeue(context.Background(), JobTypeSMSSend); err != nil {
		t.Fatalf("failed to dequeue job: %v", err)
	}

	if err := queue.Ack(context.Background(), job.ID); err != nil {
		t.Fatalf("failed to ack job: %v", err)
	}

	jobKey := "test:job:" + job.ID
	if mr.Exists(jobKey) {
		t.Error("job key should be deleted after ack")
	}
}

func TestNack(t *testing.T) {
	queue, mr := setupTestQueue(t)
	defer mr.Close()

	payload, _ := json.Marshal(map[string]string{"foo": "bar"})
	job := &Job{
		Type:     JobTypeSMSSend,
		Priority: PriorityNormal,
		Payload:  payload,
		MaxRetry: 3,
	}

	if err := queue.Enqueue(context.Background(), job); err != nil {
		t.Fatalf("failed to enqueue job: %v", err)
	}

	_, err := queue.Dequeue(context.Background(), JobTypeSMSSend)
	if err != nil {
		t.Fatalf("failed to dequeue job: %v", err)
	}

	if err := queue.Nack(context.Background(), job.ID, fmt.Errorf("test error")); err != nil {
		t.Fatalf("failed to nack job: %v", err)
	}

	delayedKey := "test:delayed:" + JobTypeSMSSend
	if !mr.Exists(delayedKey) {
		t.Error("job should be in delayed queue after nack")
	}
}

func TestNackMovesToDLQ(t *testing.T) {
	queue, mr := setupTestQueue(t)
	defer mr.Close()

	payload, _ := json.Marshal(map[string]string{"foo": "bar"})
	job := &Job{
		Type:     JobTypeSMSSend,
		Priority: PriorityNormal,
		Payload:  payload,
		MaxRetry: 2,
		Attempts: 2,
	}

	if err := queue.Enqueue(context.Background(), job); err != nil {
		t.Fatalf("failed to enqueue job: %v", err)
	}

	_, err := queue.Dequeue(context.Background(), JobTypeSMSSend)
	if err != nil {
		t.Fatalf("failed to dequeue job: %v", err)
	}

	if err := queue.Nack(context.Background(), job.ID, fmt.Errorf("test error")); err != nil {
		t.Fatalf("failed to nack job: %v", err)
	}

	dlqKey := "test:dlq:" + JobTypeSMSSend
	if !mr.Exists(dlqKey) {
		t.Error("job should be in DLQ after max retries")
	}
}

func TestProcessDelayedJobs(t *testing.T) {
	queue, mr := setupTestQueue(t)
	defer mr.Close()

	payload, _ := json.Marshal(map[string]string{"foo": "bar"})
	job := &Job{
		Type:     JobTypeSMSSend,
		Priority: PriorityNormal,
		Payload:  payload,
		MaxRetry: 3,
	}

	if err := queue.Enqueue(context.Background(), job); err != nil {
		t.Fatalf("failed to enqueue job: %v", err)
	}

	_, err := queue.Dequeue(context.Background(), JobTypeSMSSend)
	if err != nil {
		t.Fatalf("failed to dequeue job: %v", err)
	}

	if err := queue.Nack(context.Background(), job.ID, fmt.Errorf("test error")); err != nil {
		t.Fatalf("failed to nack job: %v", err)
	}

	delayedKey := "test:delayed:" + JobTypeSMSSend
	if !mr.Exists(delayedKey) {
		t.Fatal("job should be in delayed queue after nack")
	}

	pastTime := float64(time.Now().Add(-10 * time.Second).Unix())
	mr.ZAdd(delayedKey, pastTime, job.ID)

	if err := queue.ProcessDelayedJobs(context.Background()); err != nil {
		t.Fatalf("failed to process delayed jobs: %v", err)
	}

	job2, err := queue.Dequeue(context.Background(), JobTypeSMSSend)
	if err != nil {
		t.Fatalf("failed to dequeue re-queued job: %v", err)
	}
	if job2 == nil {
		t.Error("job should be moved back to main queue after delay")
	}
}
