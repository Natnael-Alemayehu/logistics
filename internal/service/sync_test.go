package service

import (
	"errors"
	"testing"
)

func TestSyncResultClassification(t *testing.T) {
	base := SyncResult{ClientID: "c1", Kind: "status", OK: true}
	if r := resultFor(base, nil); !r.OK || r.Retry {
		t.Errorf("success: %+v", r)
	}
	if r := resultFor(base, ErrStaleUpdate); r.OK || r.Retry || r.Code != "STALE" {
		t.Errorf("stale: %+v", r)
	}
	if r := resultFor(base, conflict("no")); r.OK || r.Retry || r.Code != "CONFLICT" {
		t.Errorf("domain error should be final: %+v", r)
	}
	if r := resultFor(base, errors.New("connection reset")); r.OK || !r.Retry {
		t.Errorf("internal error should be retried: %+v", r)
	}
}
