package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/natnael-alemayehu/logistics/pkg/metrics"
	"github.com/natnael-alemayehu/logistics/pkg/redis"
	goRedis "github.com/redis/go-redis/v9"
)

type RedisQueue struct {
	client    *redis.Client
	namespace string
}

func NewRedisQueue(client *redis.Client, namespace string) *RedisQueue {
	return &RedisQueue{
		client:    client,
		namespace: namespace,
	}
}

func (q *RedisQueue) Enqueue(ctx context.Context, job *Job) error {
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}
	if job.MaxRetry == 0 {
		job.MaxRetry = 3
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	queueKey := fmt.Sprintf("%s:queue:%s", q.namespace, job.Type)
	jobKey := fmt.Sprintf("%s:job:%s", q.namespace, job.ID)
	score := float64(-job.Priority)

	pipe := q.client.Raw().Pipeline()
	pipe.ZAdd(ctx, queueKey, goRedis.Z{Score: score, Member: job.ID})
	pipe.HSet(ctx, jobKey, "data", string(data))
	pipe.Expire(ctx, jobKey, 24*time.Hour)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	metrics.QueueJobsPending.WithLabelValues(job.Type).Inc()

	return nil
}

func (q *RedisQueue) Dequeue(ctx context.Context, jobTypes ...string) (*Job, error) {
	for _, jobType := range jobTypes {
		queueKey := fmt.Sprintf("%s:queue:%s", q.namespace, jobType)
		processingKey := fmt.Sprintf("%s:processing:%s", q.namespace, jobType)

		results, err := q.client.Raw().ZPopMin(ctx, queueKey, 1).Result()
		if err != nil || len(results) == 0 {
			continue
		}

		jobID := results[0].Member.(string)
		jobKey := fmt.Sprintf("%s:job:%s", q.namespace, jobID)

		data, err := q.client.Raw().HGet(ctx, jobKey, "data").Result()
		if err != nil {
			continue
		}

		var job Job
		if err := json.Unmarshal([]byte(data), &job); err != nil {
			continue
		}

		q.client.Raw().HSet(ctx, jobKey, "started_at", time.Now().Unix())
		q.client.Raw().ZAdd(ctx, processingKey, goRedis.Z{Score: float64(time.Now().Unix()), Member: job.ID})

		metrics.QueueJobsPending.WithLabelValues(jobType).Dec()

		return &job, nil
	}

	return nil, nil
}

func (q *RedisQueue) Ack(ctx context.Context, jobID string) error {
	jobKey := fmt.Sprintf("%s:job:%s", q.namespace, jobID)

	data, err := q.client.Raw().HGet(ctx, jobKey, "data").Result()
	if err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	var job Job
	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return fmt.Errorf("failed to unmarshal job: %w", err)
	}

	processingKey := fmt.Sprintf("%s:processing:%s", q.namespace, job.Type)

	pipe := q.client.Raw().Pipeline()
	pipe.ZRem(ctx, processingKey, jobID)
	pipe.Del(ctx, jobKey)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to ack job: %w", err)
	}

	metrics.QueueJobsProcessed.WithLabelValues(job.Type, "success").Inc()

	return nil
}

func (q *RedisQueue) Nack(ctx context.Context, jobID string, jobErr error) error {
	jobKey := fmt.Sprintf("%s:job:%s", q.namespace, jobID)

	data, err := q.client.Raw().HGet(ctx, jobKey, "data").Result()
	if err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	var job Job
	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return fmt.Errorf("failed to unmarshal job: %w", err)
	}

	processingKey := fmt.Sprintf("%s:processing:%s", q.namespace, job.Type)
	q.client.Raw().ZRem(ctx, processingKey, jobID)

	job.Attempts++
	if jobErr != nil {
		job.Error = jobErr.Error()
	}

	if job.Attempts >= job.MaxRetry {
		return q.moveToDLQ(ctx, &job)
	}

	backoff := time.Duration(job.Attempts*job.Attempts) * time.Second
	if backoff > 5*time.Minute {
		backoff = 5 * time.Minute
	}

	delayedQueueKey := fmt.Sprintf("%s:delayed:%s", q.namespace, job.Type)
	executeAt := time.Now().Add(backoff)

	jobData, _ := json.Marshal(job)

	pipe := q.client.Raw().Pipeline()
	pipe.HSet(ctx, jobKey, "data", string(jobData))
	pipe.ZAdd(ctx, delayedQueueKey, goRedis.Z{Score: float64(executeAt.Unix()), Member: job.ID})
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to re-enqueue job: %w", err)
	}

	metrics.QueueJobsProcessed.WithLabelValues(job.Type, "retry").Inc()

	return nil
}

func (q *RedisQueue) moveToDLQ(ctx context.Context, job *Job) error {
	dlqKey := fmt.Sprintf("%s:dlq:%s", q.namespace, job.Type)
	jobKey := fmt.Sprintf("%s:job:%s", q.namespace, job.ID)

	jobData, _ := json.Marshal(job)

	pipe := q.client.Raw().Pipeline()
	pipe.ZAdd(ctx, dlqKey, goRedis.Z{Score: float64(time.Now().Unix()), Member: job.ID})
	pipe.HSet(ctx, jobKey, "data", string(jobData))
	pipe.HSet(ctx, jobKey, "failed_at", time.Now().Unix())
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to move job to DLQ: %w", err)
	}

	metrics.QueueJobsProcessed.WithLabelValues(job.Type, "failed").Inc()

	return nil
}

func (q *RedisQueue) Close() error {
	return nil
}

func (q *RedisQueue) ProcessDelayedJobs(ctx context.Context) error {
	jobTypes := []string{
		JobTypePhotoCompress,
		JobTypeSMSSend,
		JobTypeSMSBatch,
		JobTypeReportGen,
		JobTypeDataArchive,
	}

	now := time.Now().Unix()

	for _, jobType := range jobTypes {
		delayedKey := fmt.Sprintf("%s:delayed:%s", q.namespace, jobType)
		queueKey := fmt.Sprintf("%s:queue:%s", q.namespace, jobType)

		results, err := q.client.Raw().ZRangeByScore(ctx, delayedKey, &goRedis.ZRangeBy{
			Min: "-inf",
			Max: fmt.Sprintf("%d", now),
		}).Result()
		if err != nil {
			continue
		}

		for _, jobID := range results {
			jobKey := fmt.Sprintf("%s:job:%s", q.namespace, jobID)

			data, err := q.client.Raw().HGet(ctx, jobKey, "data").Result()
			if err != nil {
				continue
			}

			var job Job
			if err := json.Unmarshal([]byte(data), &job); err != nil {
				continue
			}

			jobData, _ := json.Marshal(job)
			score := float64(-job.Priority)

			pipe := q.client.Raw().Pipeline()
			pipe.ZRem(ctx, delayedKey, jobID)
			pipe.ZAdd(ctx, queueKey, goRedis.Z{Score: score, Member: jobID})
			pipe.HSet(ctx, jobKey, "data", string(jobData))
			pipe.Exec(ctx)

			metrics.QueueJobsPending.WithLabelValues(jobType).Inc()
		}
	}

	return nil
}
