// Package monitor ties the APRS-IS feed, the silence tracker and the
// notifiers together.
package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/MikeWills/aprsmonitor/internal/gate"
	"github.com/MikeWills/aprsmonitor/internal/notify"
	"github.com/MikeWills/aprsmonitor/internal/watch"
)

// Config for a Monitor.
type Config struct {
	Callsign        string
	Threshold       time.Duration
	CountOwnPackets bool
	NotifyRecovery  bool
	Notifiers       []notify.Notifier

	Logger     *slog.Logger
	Now        func() time.Time // defaults to time.Now
	RetryDelay time.Duration    // first retry delay for failed notifications (default 2s)
}

// Monitor is safe for concurrent use: packets arrive on the APRS-IS goroutine
// while Tick runs on a timer.
type Monitor struct {
	cfg     Config
	matcher *gate.Matcher

	mu        sync.Mutex
	tracker   *watch.Tracker
	connected bool
	lastHeard time.Time // zero until a gated packet is heard
	downSince time.Time
}

func New(cfg Config) *Monitor {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 2 * time.Second
	}
	return &Monitor{
		cfg:     cfg,
		matcher: gate.New(cfg.Callsign, cfg.CountOwnPackets),
		tracker: watch.New(cfg.Threshold, cfg.Now()),
	}
}

// OnConnect is called when the APRS-IS connection is established.
func (m *Monitor) OnConnect() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
	m.tracker.Resume(m.cfg.Now())
}

// OnDisconnect is called when the APRS-IS connection is lost. While
// disconnected the monitor cannot judge the iGate and raises no alert.
func (m *Monitor) OnDisconnect(error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
}

// OnPacket is called for every packet delivered by APRS-IS.
func (m *Monitor) OnPacket(raw string) {
	if !m.matcher.Heard(raw) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.cfg.Now()
	m.tracker.Heard(now)
	m.lastHeard = now
}

// Tick evaluates the tracker and sends any resulting alert.
func (m *Monitor) Tick(ctx context.Context) {
	m.mu.Lock()
	if !m.connected {
		m.mu.Unlock()
		return
	}
	now := m.cfg.Now()
	ev := m.tracker.Check(now)
	last := m.lastHeard
	var msg notify.Message
	switch ev {
	case watch.Down:
		m.downSince = now
		msg = m.downMessage(now, last)
	case watch.Recovered:
		msg = m.recoveredMessage(now, last)
	}
	m.mu.Unlock()

	if ev == watch.None || (ev == watch.Recovered && !m.cfg.NotifyRecovery) {
		return
	}
	m.cfg.Logger.Warn("state change", "event", msg.Event, "callsign", m.cfg.Callsign)
	m.send(ctx, msg)
}

// SendTest sends a test notification so the owner can verify delivery.
func (m *Monitor) SendTest(ctx context.Context) error {
	return notify.Dispatch(ctx, m.cfg.Notifiers, notify.Message{
		Event:    "test",
		Callsign: m.cfg.Callsign,
		Subject:  "AprsMonitor test alert for " + m.cfg.Callsign,
		Body:     "This is a test. If you can read this, alerts for " + m.cfg.Callsign + " will reach you.",
	}, 1, m.cfg.RetryDelay)
}

// Run calls Tick every interval until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.Tick(ctx)
		}
	}
}

func (m *Monitor) send(ctx context.Context, msg notify.Message) {
	if err := notify.Dispatch(ctx, m.cfg.Notifiers, msg, 3, m.cfg.RetryDelay); err != nil {
		m.cfg.Logger.Error("notification failed", "event", msg.Event, "err", err)
	}
}

func (m *Monitor) downMessage(now, last time.Time) notify.Message {
	since := "since the monitor started or last reconnected"
	if !last.IsZero() {
		since = "last heard " + last.UTC().Format("2006-01-02 15:04 UTC")
	}
	return notify.Message{
		Event:    "down",
		Callsign: m.cfg.Callsign,
		Subject:  fmt.Sprintf("APRS iGate %s appears DOWN", m.cfg.Callsign),
		Body: fmt.Sprintf("No packets gated by %s have appeared on APRS-IS for more than %s (%s).\n\n"+
			"Please check the station: power, internet connection, radio/receiver and the iGate software.\n"+
			"If the band is simply quiet, consider raising silence_threshold.",
			m.cfg.Callsign, m.cfg.Threshold, since),
	}
}

func (m *Monitor) recoveredMessage(now, last time.Time) notify.Message {
	down := ""
	if !m.downSince.IsZero() {
		down = fmt.Sprintf(" It was reported down %s ago.", now.Sub(m.downSince).Round(time.Minute))
	}
	return notify.Message{
		Event:    "recovered",
		Callsign: m.cfg.Callsign,
		Subject:  fmt.Sprintf("APRS iGate %s is back", m.cfg.Callsign),
		Body:     fmt.Sprintf("A packet gated by %s was just heard on APRS-IS.%s", m.cfg.Callsign, down),
	}
}
