package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// liveChannel is the Postgres channel live updates cross between
// instances on.
const liveChannel = "live_updates"

// maxNotify is the most a NOTIFY may carry, a little under Postgres's 8000
// bytes. A live update is a few hundred.
const maxNotify = 7900

// localHub is where an instance's own dashboards are connected.
type localHub interface {
	BroadcastToTenant(tenantID string, message []byte)
}

// liveRelay sends live updates through Postgres, so that a change made on
// one instance reaches the dashboards connected to every other (audit
// O4). Each instance listens, and hands what it hears to its own hub; it
// hears its own updates too, so it sends nothing to the hub directly.
type liveRelay struct {
	pool *pgxpool.Pool
	hub  localHub
	log  zerolog.Logger
}

type liveNote struct {
	Tenant  string          `json:"t"`
	Message json.RawMessage `json:"m"`
}

// BroadcastToTenant publishes an update for every instance's dashboards
// of a company. It is called after a change commits, so it waits on the
// database a moment at most; if Postgres can't take it, this instance's
// own dashboards still get it.
func (l *liveRelay) BroadcastToTenant(tenantID string, message []byte) {
	payload, err := json.Marshal(liveNote{Tenant: tenantID, Message: message})
	if err != nil || len(payload) > maxNotify {
		l.log.Warn().Int("bytes", len(payload)).Msg("live update too large to share between instances")
		l.hub.BroadcastToTenant(tenantID, message)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := l.pool.Exec(ctx, "SELECT pg_notify($1, $2)", liveChannel, string(payload)); err != nil {
		l.log.Warn().Err(err).Msg("publish live update")
		l.hub.BroadcastToTenant(tenantID, message)
	}
}

// Run listens for live updates until ctx ends, on a connection of its
// own, reconnecting when it is lost. Updates published while it is away
// are missed: an open page catches up at its next change, or when its own
// WebSocket reconnects.
func (l *liveRelay) Run(ctx context.Context) {
	delay := time.Second
	for ctx.Err() == nil {
		err := l.listen(ctx)
		if ctx.Err() != nil {
			return
		}
		l.log.Warn().Err(err).Dur("retry_in", delay).Msg("live updates: listener lost")
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(delay*2, 30*time.Second)
	}
}

func (l *liveRelay) listen(ctx context.Context) error {
	pc, err := l.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	// The connection leaves the pool for good: a listening connection
	// can't be handed to anyone else.
	conn := pc.Hijack()
	defer conn.Close(context.Background()) //nolint:errcheck // closing a connection we are done with
	if _, err := conn.Exec(ctx, "LISTEN "+liveChannel); err != nil {
		return err
	}
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var note liveNote
		if err := json.Unmarshal([]byte(n.Payload), &note); err != nil || note.Tenant == "" {
			l.log.Warn().Msg("live updates: a notification that isn't one")
			continue
		}
		l.hub.BroadcastToTenant(note.Tenant, note.Message)
	}
}
