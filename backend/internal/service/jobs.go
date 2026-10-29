package service

import (
	"context"
	"fmt"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/db"
)

// Jobs are the periodic checks: drivers gone quiet, shipments past their ETA,
// and data retention. Each run takes a Postgres advisory lock, so running
// several replicas does not raise every alert several times.
type Jobs struct{ *core }

const (
	jobsLockKey = 7_311_042 // arbitrary, unique to this application

	driverOfflineAfter = 4 * time.Hour

	retainTracking = 90 * 24 * time.Hour
	retainAlerts   = 90 * 24 * time.Hour
	retainSMS      = 180 * 24 * time.Hour
	retainEmail    = 180 * 24 * time.Hour
)

// Run performs the checks every minute and retention every hour until ctx
// ends.
func (j *Jobs) Run(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	var lastRetention time.Time
	for {
		retention := time.Since(lastRetention) > time.Hour
		if err := j.once(ctx, retention); err != nil && ctx.Err() == nil {
			j.log.Error().Err(err).Msg("periodic jobs")
		} else if retention {
			lastRetention = time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (j *Jobs) once(ctx context.Context, retention bool) error {
	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // releases the lock
	var got bool
	if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", jobsLockKey).Scan(&got); err != nil {
		return err
	}
	if !got {
		return nil
	}

	if err := j.CheckDrivers(ctx); err != nil {
		return fmt.Errorf("driver check: %w", err)
	}
	if err := j.CheckETAs(ctx); err != nil {
		return fmt.Errorf("eta check: %w", err)
	}
	if err := j.q.RequeueStuckSMS(ctx); err != nil {
		return fmt.Errorf("requeue sms: %w", err)
	}
	if err := j.q.RequeueStuckEmails(ctx); err != nil {
		return fmt.Errorf("requeue email: %w", err)
	}
	if retention {
		if err := j.Retention(ctx); err != nil {
			return fmt.Errorf("retention: %w", err)
		}
	}
	return nil
}

// CheckDrivers alerts on drivers carrying cargo who have not been heard from
// in four hours (FR-033).
func (j *Jobs) CheckDrivers(ctx context.Context) error {
	now := j.now()
	stale, err := j.q.ListStaleActiveDrivers(ctx, toTimestamp(now.Add(-driverOfflineAfter)))
	if err != nil {
		return err
	}
	if len(stale) == 0 {
		return nil
	}
	return j.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		for _, d := range stale {
			j.raise(ctx, q, after, newAlert{
				TenantID: d.TenantID, DriverID: d.DriverID, Type: AlertDriverOffline, Severity: SeverityWarning,
				Message: fmt.Sprintf("%s has not been in touch for %s while carrying %s.",
					d.DriverName, humanDuration(now.Sub(d.LastContact.Time)), d.TrackingNumber),
			})
		}
		return nil
	})
}

// CheckETAs alerts on active shipments past their estimated delivery.
func (j *Jobs) CheckETAs(ctx context.Context) error {
	late, err := j.q.ListOverdueShipments(ctx)
	if err != nil {
		return err
	}
	if len(late) == 0 {
		return nil
	}
	return j.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		for _, s := range late {
			j.raise(ctx, q, after, newAlert{
				TenantID: s.TenantID, ShipmentID: s.ID, DriverID: s.DriverID, Type: AlertETAExceeded, Severity: SeverityWarning,
				Message: fmt.Sprintf("%s is past its estimated delivery of %s.", s.TrackingNumber, j.formatWhen(s.EstimatedDelivery.Time)),
			})
		}
		return nil
	})
}

// Retention deletes data past its keeping period (NFR data retention).
func (j *Jobs) Retention(ctx context.Context) error {
	now := j.now()
	if err := j.q.DeleteOldTrackingEvents(ctx, toTimestamp(now.Add(-retainTracking))); err != nil {
		return err
	}
	if err := j.q.DeleteOldAlerts(ctx, toTimestamp(now.Add(-retainAlerts))); err != nil {
		return err
	}
	if err := j.q.DeleteOldSMS(ctx, toTimestamp(now.Add(-retainSMS))); err != nil {
		return err
	}
	if err := j.q.DeleteOldEmails(ctx, toTimestamp(now.Add(-retainEmail))); err != nil {
		return err
	}
	if err := j.q.DeleteExpiredSessions(ctx); err != nil {
		return err
	}
	return j.q.DeleteExpiredVerificationCodes(ctx)
}

// humanDuration renders a duration the way a dispatcher would say it.
func humanDuration(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	case d < 48*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "an hour"
		}
		return fmt.Sprintf("%d hours", h)
	default:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	}
}
