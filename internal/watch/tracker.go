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
	lastHeard time.Time
	down      bool
}

// New returns a Tracker that reports Down after threshold of silence.
func New(threshold time.Duration) *Tracker {
	return &Tracker{threshold: threshold}
}

// Heard records a packet gated by the watched station at time t.
func (t *Tracker) Heard(at time.Time) { t.lastHeard = at }

// Check returns the state change, if any, as of now.
func (t *Tracker) Check(now time.Time) Event {
	silent := now.Sub(t.lastHeard) > t.threshold
	switch {
	case silent && !t.down:
		t.down = true
		return Down
	case !silent && t.down:
		t.down = false
		return Recovered
	}
	return None
}
