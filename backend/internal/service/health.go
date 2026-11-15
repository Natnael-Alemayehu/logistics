package service

import (
	"context"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/pkg/metrics"
)

// HealthService reports how the system is running, for the platform admin
// (FR-049 AC-049.6).
type HealthService struct{ *core }

// The background jobs, in the order they run, with how often.
var jobSchedule = []struct {
	Name, Label string
	Every       time.Duration
}{
	{"driver_check", "Drivers gone quiet", time.Minute},
	{"eta_check", "Shipments past their due time", time.Minute},
	{"pickup_check", "Late pickups", time.Minute},
	{"route_check", "Trucks heading away from the destination", time.Minute},
	{"outbox_requeue", "Stuck texts and emails", time.Minute},
	{"billing", "Renewals, reminders and read-only", time.Minute},
	{"plan_limits", "Plan limits", usageEvery},
	{"retention", "Data retention", time.Hour},
}

// Health is a snapshot of the system.
type Health struct {
	DBLatency time.Duration
	DBSize    int64
	// Migration is the newest schema migration applied to the database.
	Migration                      int64
	PoolTotal, PoolIdle, PoolInUse int32
	Jobs                           []JobHealth
	Outbox                         []db.OutboxHealthRow
	WaitingTickets                 int64
}

// JobHealth is one background job's latest run.
type JobHealth struct {
	Name, Label string
	Every       time.Duration
	Run         *db.JobRun
	// Late: it hasn't run for three times its interval, so no replica is
	// running the jobs.
	Late bool
}

// Check measures the database and reads the jobs' and outboxes' state.
func (s *HealthService) Check(ctx context.Context, p *Principal) (*Health, error) {
	if !p.IsPlatformAdmin() {
		return nil, ErrUnauthorized
	}
	h := &Health{}
	start := time.Now()
	var one int
	if err := s.pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		return nil, err
	}
	h.DBLatency = time.Since(start)
	if err := s.pool.QueryRow(ctx, "SELECT pg_database_size(current_database())").Scan(&h.DBSize); err != nil {
		return nil, err
	}
	if err := s.pool.QueryRow(ctx, "SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied").Scan(&h.Migration); err != nil {
		return nil, err
	}
	st := s.pool.Stat()
	h.PoolTotal, h.PoolIdle, h.PoolInUse = st.TotalConns(), st.IdleConns(), st.AcquiredConns()

	runs, err := s.q.ListJobRuns(ctx)
	if err != nil {
		return nil, err
	}
	byName := map[string]*db.JobRun{}
	for i := range runs {
		byName[runs[i].Name] = &runs[i]
	}
	now := s.now()
	for _, j := range jobSchedule {
		r := byName[j.Name]
		late := r == nil || now.Sub(r.StartedAt.Time) > 3*j.Every+time.Minute
		h.Jobs = append(h.Jobs, JobHealth{Name: j.Name, Label: j.Label, Every: j.Every, Run: r, Late: late})
	}
	if h.Outbox, err = s.q.OutboxHealth(ctx); err != nil {
		return nil, err
	}
	h.WaitingTickets, err = s.q.CountWaitingTickets(ctx)
	return h, err
}

// record runs one job and keeps its outcome for the health panel.
func (j *Jobs) record(ctx context.Context, name string, fn func(context.Context) error) error {
	start, begun := j.now(), time.Now()
	err := fn(ctx)
	metrics.JobDuration.WithLabelValues(name).Observe(time.Since(begun).Seconds())
	var msg *string
	if err != nil {
		m := truncate(err.Error(), 500)
		msg = &m
		metrics.JobFailures.WithLabelValues(name).Inc()
	} else {
		metrics.JobLastSuccess.WithLabelValues(name).SetToCurrentTime()
	}
	if rerr := j.q.RecordJobRun(ctx, db.RecordJobRunParams{
		Name: name, StartedAt: toTimestamp(start), DurationMs: int32(time.Since(begun).Milliseconds()), Error: msg,
	}); rerr != nil && ctx.Err() == nil {
		j.log.Error().Err(rerr).Str("job", name).Msg("record job run")
	}
	return err
}
