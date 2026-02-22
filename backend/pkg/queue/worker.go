package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type Worker struct {
	queue       Queue
	handlers    map[string]JobHandler
	concurrency int
	logger      zerolog.Logger
	stop        chan struct{}
	wg          sync.WaitGroup
}

func NewWorker(queue Queue, concurrency int, logger zerolog.Logger) *Worker {
	return &Worker{
		queue:       queue,
		handlers:    make(map[string]JobHandler),
		concurrency: concurrency,
		logger:      logger,
		stop:        make(chan struct{}),
	}
}

func (w *Worker) RegisterHandler(jobType string, handler JobHandler) {
	w.handlers[jobType] = handler
}

func (w *Worker) Start() {
	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.run()
	}
}

func (w *Worker) Stop() {
	close(w.stop)
	w.wg.Wait()
}

func (w *Worker) run() {
	defer w.wg.Done()

	jobTypes := []string{
		JobTypePhotoCompress,
		JobTypeSMSSend,
		JobTypeSMSBatch,
		JobTypeReportGen,
		JobTypeDataArchive,
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			job, err := w.queue.Dequeue(context.Background(), jobTypes...)
			if err != nil {
				w.logger.Error().Err(err).Msg("failed to dequeue job")
				continue
			}
			if job == nil {
				continue
			}
			w.process(context.Background(), job)
		}
	}
}

func (w *Worker) process(ctx context.Context, job *Job) {
	handler, ok := w.handlers[job.Type]
	if !ok {
		w.logger.Error().Str("job_type", job.Type).Str("job_id", job.ID).Msg("no handler registered for job type")
		w.queue.Nack(ctx, job.ID, fmt.Errorf("no handler registered for job type: %s", job.Type))
		return
	}

	timeout := 5 * time.Minute
	if job.Type == JobTypeReportGen {
		timeout = 30 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	w.logger.Info().Str("job_id", job.ID).Str("job_type", job.Type).Int("attempt", job.Attempts).Msg("processing job")

	if err := handler(ctx, job); err != nil {
		w.logger.Error().Err(err).Str("job_id", job.ID).Str("job_type", job.Type).Msg("job failed")
		if nackErr := w.queue.Nack(ctx, job.ID, err); nackErr != nil {
			w.logger.Error().Err(nackErr).Str("job_id", job.ID).Msg("failed to nack job")
		}
		return
	}

	if err := w.queue.Ack(ctx, job.ID); err != nil {
		w.logger.Error().Err(err).Str("job_id", job.ID).Msg("failed to ack job")
	}
}
