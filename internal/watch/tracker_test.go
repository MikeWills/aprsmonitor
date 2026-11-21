package watch

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func TestTrackerAlertsAfterSilenceThenRecovers(t *testing.T) {
	tr := New(30*time.Minute, t0)
	tr.Heard(t0)

	if ev := tr.Check(t0.Add(29 * time.Minute)); ev != None {
		t.Fatalf("before threshold: got %v, want None", ev)
	}
	if ev := tr.Check(t0.Add(31 * time.Minute)); ev != Down {
		t.Fatalf("after threshold: got %v, want Down", ev)
	}
	if ev := tr.Check(t0.Add(40 * time.Minute)); ev != None {
		t.Fatalf("already alerted: got %v, want None (no repeat)", ev)
	}
	tr.Heard(t0.Add(41 * time.Minute))
	if ev := tr.Check(t0.Add(42 * time.Minute)); ev != Recovered {
		t.Fatalf("after heard: got %v, want Recovered", ev)
	}
}

// Starting the monitor must not alert instantly: the clock starts at New.
func TestTrackerStartupGracePeriod(t *testing.T) {
	tr := New(30*time.Minute, t0)
	if ev := tr.Check(t0.Add(time.Minute)); ev != None {
		t.Fatalf("just started, never heard: got %v, want None", ev)
	}
	if ev := tr.Check(t0.Add(31 * time.Minute)); ev != Down {
		t.Fatalf("never heard for > threshold: got %v, want Down", ev)
	}
}

// Time spent disconnected from APRS-IS says nothing about the iGate.
func TestTrackerResumeDoesNotCountDisconnectedTime(t *testing.T) {
	tr := New(30*time.Minute, t0)
	tr.Heard(t0)
	// monitor offline from t0+5m to t0+60m, then reconnects
	tr.Resume(t0.Add(60 * time.Minute))
	if ev := tr.Check(t0.Add(61 * time.Minute)); ev != None {
		t.Fatalf("right after reconnect: got %v, want None", ev)
	}
	if ev := tr.Check(t0.Add(91 * time.Minute)); ev != Down {
		t.Fatalf("30m of connected silence: got %v, want Down", ev)
	}
}

// Reconnecting while Down must not fake a recovery.
func TestTrackerResumeWhileDownIsNotRecovery(t *testing.T) {
	tr := New(30*time.Minute, t0)
	if ev := tr.Check(t0.Add(31 * time.Minute)); ev != Down {
		t.Fatalf("got %v, want Down", ev)
	}
	tr.Resume(t0.Add(50 * time.Minute))
	if ev := tr.Check(t0.Add(51 * time.Minute)); ev != None {
		t.Fatalf("reconnected but nothing heard: got %v, want None", ev)
	}
	if ev := tr.Check(t0.Add(81 * time.Minute)); ev != None {
		t.Fatalf("already alerted, still silent: got %v, want None", ev)
	}
	tr.Heard(t0.Add(82 * time.Minute))
	if ev := tr.Check(t0.Add(83 * time.Minute)); ev != Recovered {
		t.Fatalf("got %v, want Recovered", ev)
	}
}
