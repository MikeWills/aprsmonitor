// Package watch tracks when an iGate was last heard and reports state changes.
package watch

import "time"

// Event is a state change reported by Check.
type Event int

const (
	None      Event = iota // nothing to report
	Down                   // silent longer than the threshold (reported once)
	Recovered              // heard again after being reported Down
)

func (e Event) String() string {
	return [...]string{"None", "Down", "Recovered"}[e]
}

// Tracker is not safe for concurrent use; guard it with a mutex if shared.
type Tracker struct {
	threshold time.Duration
	// clock is the start of the current silence measurement: the later of the
	// last heard packet, the last (re)connect, and the tracker's creation.
	clock time.Time
	down  bool
	// heardSinceDown is set by Heard and is the only thing that ends Down.
	heardSinceDown bool
}

// New returns a Tracker that reports Down after threshold of silence. The
// silence clock starts at start, so a fresh monitor gets a full threshold of
// grace before it can alert.
func New(threshold time.Duration, start time.Time) *Tracker {
	return &Tracker{threshold: threshold, clock: start}
}

// Heard records a packet gated by the watched station at time at.
func (t *Tracker) Heard(at time.Time) {
	if at.After(t.clock) {
		t.clock = at
	}
	t.heardSinceDown = true
}

// Resume must be called when the monitor (re)connects to APRS-IS. Time spent
// disconnected is not evidence about the iGate, so the silence clock restarts
// at now. It never changes the Down state; only Heard can recover.
func (t *Tracker) Resume(now time.Time) {
	if now.After(t.clock) {
		t.clock = now
	}
}

// Check returns the state change, if any, as of now.
func (t *Tracker) Check(now time.Time) Event {
	if t.down {
		if t.heardSinceDown {
			t.down = false
			return Recovered
		}
		return None
	}
	t.heardSinceDown = false
	if now.Sub(t.clock) > t.threshold {
		t.down = true
		t.heardSinceDown = false
		return Down
	}
	return None
}
