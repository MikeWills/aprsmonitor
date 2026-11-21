package monitor

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"aprsmonitor/internal/notify"
)

type capture struct {
	mu   sync.Mutex
	msgs []notify.Message
}

func (c *capture) Name() string { return "capture" }
func (c *capture) Send(_ context.Context, m notify.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, m)
	return nil
}
func (c *capture) events() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var e []string
	for _, m := range c.msgs {
		e = append(e, m.Event)
	}
	return e
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

const gated = "N0CALL>APRS,WIDE1-1,qAR,W0MIK-10:>status"
const otherGated = "N0CALL>APRS,qAR,K0ABC-5:>status"

func setup(notifyRecovery bool) (*Monitor, *capture, *clock) {
	cap := &capture{}
	clk := &clock{t: time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)}
	m := New(Config{
		Callsign: "W0MIK-10", Threshold: time.Hour, NotifyRecovery: notifyRecovery,
		Notifiers: []notify.Notifier{cap}, Now: clk.now, RetryDelay: time.Millisecond,
	})
	m.OnConnect()
	return m, cap, clk
}

func TestAlertsOnceThenRecovers(t *testing.T) {
	m, cap, clk := setup(true)
	ctx := context.Background()

	clk.advance(30 * time.Minute)
	m.Tick(ctx)
	if len(cap.events()) != 0 {
		t.Fatalf("alerted too early: %v", cap.events())
	}
	clk.advance(31 * time.Minute)
	m.Tick(ctx)
	m.Tick(ctx) // must not repeat
	if got := cap.events(); len(got) != 1 || got[0] != "down" {
		t.Fatalf("events = %v, want [down]", got)
	}
	if s := cap.msgs[0].Subject; !strings.Contains(s, "W0MIK-10") {
		t.Errorf("subject %q should name the station", s)
	}

	m.OnPacket(gated)
	m.Tick(ctx)
	if got := cap.events(); len(got) != 2 || got[1] != "recovered" {
		t.Fatalf("events = %v, want [down recovered]", got)
	}
}

func TestIgnoresOtherStationsPackets(t *testing.T) {
	m, cap, clk := setup(true)
	for i := 0; i < 5; i++ {
		clk.advance(15 * time.Minute)
		m.OnPacket(otherGated)
		m.Tick(context.Background())
	}
	if got := cap.events(); len(got) != 1 || got[0] != "down" {
		t.Fatalf("events = %v, want [down] (other stations' packets are not proof of life)", got)
	}
}

func TestGatedPacketsKeepItQuiet(t *testing.T) {
	m, cap, clk := setup(true)
	for i := 0; i < 10; i++ {
		clk.advance(30 * time.Minute)
		m.OnPacket(gated)
		m.Tick(context.Background())
	}
	if got := cap.events(); len(got) != 0 {
		t.Fatalf("events = %v, want none", got)
	}
}

func TestNoAlertWhileMonitorDisconnected(t *testing.T) {
	m, cap, clk := setup(true)
	m.OnDisconnect(nil)
	clk.advance(3 * time.Hour)
	m.Tick(context.Background())
	if got := cap.events(); len(got) != 0 {
		t.Fatalf("events = %v, want none while blind", got)
	}
	m.OnConnect() // silence clock restarts
	clk.advance(10 * time.Minute)
	m.Tick(context.Background())
	if got := cap.events(); len(got) != 0 {
		t.Fatalf("events = %v, want none right after reconnect", got)
	}
}

func TestRecoveryNotificationCanBeDisabled(t *testing.T) {
	m, cap, clk := setup(false)
	clk.advance(2 * time.Hour)
	m.Tick(context.Background())
	m.OnPacket(gated)
	m.Tick(context.Background())
	if got := cap.events(); len(got) != 1 || got[0] != "down" {
		t.Fatalf("events = %v, want [down] only", got)
	}
}

func TestSendTest(t *testing.T) {
	m, cap, _ := setup(true)
	if err := m.SendTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := cap.events(); len(got) != 1 || got[0] != "test" {
		t.Fatalf("events = %v, want [test]", got)
	}
}
