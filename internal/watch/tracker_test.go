package watch

import (
	"testing"
	"time"
)

func TestTrackerAlertsAfterSilenceThenRecovers(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	tr := New(30 * time.Minute)
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
