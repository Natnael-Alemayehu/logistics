//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/sandbox"
)

// A stopping app, as on a deploy, sees the text it is sending through and
// hands back the others it had taken, which the next start sends at once
// rather than after their five-minute lease.
func TestStoppingWhileSending(t *testing.T) {
	e, p := sandboxed(t, func(c *sandbox.Config) { c.Timing.Slow = 2 * time.Second })
	office := e.company("Deploy Day Freight")
	tenant := scalar[string](t, `SELECT tenant_id::text FROM users WHERE email = $1`, office.email)
	phones := []string{phoneNumber(), phoneNumber(), phoneNumber()}
	state := func() string {
		return scalar[string](t, `SELECT string_agg(status || ' ' || attempts, ', ' ORDER BY status, attempts)
			FROM sms_messages WHERE phone = ANY($1)`, phones)
	}

	// One insert, so the outbox takes all three in one batch, and a gateway
	// slow to answer, so the first is still being sent when the app stops.
	p.faults(map[string]any{"sms_slow": true})
	exec(t, `INSERT INTO sms_messages (tenant_id, direction, phone, body, purpose)
		SELECT $1, 'outbound', phone, 'Your load is on its way', 'driver_assigned' FROM unnest($2::text[]) phone`, tenant, phones)
	within(t, 10*time.Second, "the outbox taking the texts", func() bool { return state() == "sending 1, sending 1, sending 1" })
	time.Sleep(300 * time.Millisecond) // the first send is with the gateway
	stopped := time.Now()
	e.stop()
	if took := time.Since(stopped); took < time.Second {
		t.Errorf("stopped after %v, without waiting for the text being sent", took)
	}
	if got := state(); got != "queued 0, queued 0, sent 1" {
		t.Fatalf("after stopping: %s, want the text in hand sent and the others handed back untried", got)
	}
	if !scalar[bool](t, `SELECT bool_and(next_attempt_at <= NOW()) FROM sms_messages WHERE phone = ANY($1) AND status = 'queued'`, phones) {
		t.Error("the texts handed back wait out the lease")
	}

	p.faults(map[string]any{"sms_slow": false})
	e.work()
	within(t, 5*time.Second, "the next start sending them", func() bool { return state() == "sent 1, sent 1, sent 1" })
	for _, phone := range phones {
		if n := len(p.phone(phone).from(senderID)); n != 1 {
			t.Errorf("%s received %d texts, want 1", phone, n)
		}
	}
}
