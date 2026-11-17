package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/pkg/metrics"
)

// Jobs are the periodic checks: drivers gone quiet, shipments past their ETA,
// billing, and data retention. Each run takes a Postgres advisory lock, so
// running several replicas does not raise every alert several times.
type Jobs struct {
	*core
	billing *BillingService
	data    *DataService
}

const (
	jobsLockKey = 7_311_042 // arbitrary, unique to this application

	driverOfflineAfter = 4 * time.Hour
	// pickupGrace is how long after its planned pickup a load may still be
	// waiting before the office is told.
	pickupGrace = 2 * time.Hour

	retainTracking = 90 * 24 * time.Hour
	retainAlerts   = 90 * 24 * time.Hour
	retainSMS      = 180 * 24 * time.Hour //nolint:staticcheck // SMS, not milliseconds
	retainEmail    = 180 * 24 * time.Hour
	retainLinks    = 30 * 24 * time.Hour

	// usageEvery spaces out the plan-limit checks, which count every
	// company's texts this month.
	usageEvery = 10 * time.Minute
)

// Run performs the checks every minute, the plan-limit checks every ten
// minutes and retention every hour until ctx ends.
func (j *Jobs) Run(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	var lastRetention, lastUsage time.Time
	for {
		retention := time.Since(lastRetention) > time.Hour
		usage := time.Since(lastUsage) > usageEvery
		ran, err := j.once(ctx, retention, usage)
		if err != nil && ctx.Err() == nil {
			j.log.Error().Err(err).Msg("periodic jobs")
		}
		// A failed run is retried at its next turn, not every minute: the
		// health panel shows the error meanwhile.
		if ran && retention {
			lastRetention = time.Now()
		}
		if ran && usage {
			lastUsage = time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// once runs every job due, each on its own: one failing doesn't hold up the
// others. ran is false when another replica holds the lock.
func (j *Jobs) once(ctx context.Context, retention, usage bool) (ran bool, err error) {
	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // releases the lock
	var got bool
	if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", jobsLockKey).Scan(&got); err != nil {
		return false, err
	}
	if !got {
		return false, nil
	}

	var errs []error
	run := func(name string, fn func(context.Context) error) {
		if err := j.record(ctx, name, fn); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}
	run("driver_check", j.CheckDrivers)
	run("eta_check", j.CheckETAs)
	run("pickup_check", j.CheckPickups)
	run("route_check", j.CheckRoutes)
	run("outbox_requeue", func(ctx context.Context) error {
		if err := errors.Join(j.q.RequeueStuckSMS(ctx), j.q.RequeueStuckEmails(ctx), j.q.RequeueStuckWebhooks(ctx)); err != nil {
			return err
		}
		return j.observeOutbox(ctx)
	})
	run("billing", j.billing.Run)
	if usage {
		run("plan_limits", j.billing.CheckUsage)
	}
	if retention {
		run("retention", j.Retention)
	}
	return true, errors.Join(errs...)
}

// observeOutbox publishes how many texts and emails are waiting to be sent,
// and for how long, for /metrics.
func (j *Jobs) observeOutbox(ctx context.Context) error {
	rows, err := j.q.OutboxWaiting(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		metrics.OutboxWaiting.WithLabelValues(r.Channel).Set(float64(r.Waiting))
		var age float64
		if r.Oldest.Valid {
			age = time.Since(r.Oldest.Time).Seconds()
		}
		metrics.OutboxOldest.WithLabelValues(r.Channel).Set(age)
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
				Episode: d.Episode,
			})
		}
		return nil
	})
}

// CheckETAs alerts on shipments on the road past their estimated delivery,
// once per ETA, and closes alerts whose ETA has since changed or passed out
// of question.
func (j *Jobs) CheckETAs(ctx context.Context) error {
	if err := j.q.AcknowledgeStaleETAAlerts(ctx); err != nil {
		return err
	}
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
				Episode: s.Episode,
			})
		}
		return nil
	})
}

// CheckPickups alerts on loads still waiting to be collected two hours after
// their planned pickup.
func (j *Jobs) CheckPickups(ctx context.Context) error {
	if err := j.q.AcknowledgeStalePickupAlerts(ctx); err != nil {
		return err
	}
	waiting, err := j.q.ListPickupOverdue(ctx, toTimestamp(j.now().Add(-pickupGrace)))
	if err != nil {
		return err
	}
	if len(waiting) == 0 {
		return nil
	}
	return j.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		for _, s := range waiting {
			why := "no driver is assigned"
			if s.Status == model.StatusAssigned {
				why = "the driver has not set off"
			}
			j.raise(ctx, q, after, newAlert{
				TenantID: s.TenantID, ShipmentID: s.ID, DriverID: s.DriverID, Type: AlertPickupOverdue, Severity: SeverityWarning,
				Message: fmt.Sprintf("%s was due for pickup at %s and %s.", s.TrackingNumber, j.formatWhen(s.PlannedPickupAt.Time), why),
				Episode: s.Episode,
			})
		}
		return nil
	})
}

// Retention deletes data past its keeping period and removes customers'
// details from shipments finished more than a year ago (FR-041).
func (j *Jobs) Retention(ctx context.Context) error {
	now := j.now()
	if n, err := j.data.AnonymiseFinished(ctx, now.Add(-j.cfg.RetainShipments)); err != nil {
		return err
	} else if n > 0 {
		j.log.Info().Int("shipments", n).Msg("removed customer details from old shipments")
	}
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
	if err := j.q.DeleteOldWebhookDeliveries(ctx, toTimestamp(now.Add(-retainWebhooks))); err != nil {
		return err
	}
	if err := j.q.DeleteOldEmailTokens(ctx, toTimestamp(now.Add(-retainLinks))); err != nil {
		return err
	}
	if err := j.q.DeleteOldBroadcasts(ctx, toTimestamp(now.Add(-retainBroadcasts))); err != nil {
		return err
	}
	if err := j.q.DeleteExpiredPendingSignups(ctx, toTimestamp(now)); err != nil {
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
