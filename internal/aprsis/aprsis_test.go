package aprsis

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeServer accepts connections, speaks just enough APRS-IS (banner, login,
// logresp) and then calls serve. Each accepted connection gets its own call.
func fakeServer(t *testing.T, serve func(n int, login string, c net.Conn)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for n := 0; ; n++ {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(n int, c net.Conn) {
				defer c.Close()
				c.Write([]byte("# aprsc 2.1.19 fake\r\n"))
				r := bufio.NewReader(c)
				var login string
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if strings.HasPrefix(line, "user ") {
						login = strings.TrimSpace(line)
						break
					}
				}
				c.Write([]byte("# logresp W0MON unverified, server FAKE\r\n"))
				serve(n, login, c)
			}(n, c)
		}
	}()
	return ln.Addr().String()
}

type recorder struct {
	mu         sync.Mutex
	connects   int
	packets    []string
	gotPackets chan struct{}
}

func newRecorder() *recorder { return &recorder{gotPackets: make(chan struct{}, 100)} }

func (r *recorder) handler() Handler {
	return Handler{
		OnConnect: func() { r.mu.Lock(); r.connects++; r.mu.Unlock() },
		OnPacket: func(raw string) {
			r.mu.Lock()
			r.packets = append(r.packets, raw)
			r.mu.Unlock()
			r.gotPackets <- struct{}{}
		},
	}
}

func (r *recorder) waitPackets(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-r.gotPackets:
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for packet %d of %d", i+1, n)
		}
	}
}

func testConfig(addr string) Config {
	return Config{
		Server: addr, Callsign: "W0MON", Passcode: "-1", Filter: "e/W0MIK-10",
		AppVersion: "test", ReadTimeout: 2 * time.Second,
		MinBackoff: 10 * time.Millisecond, MaxBackoff: 50 * time.Millisecond,
	}
}

func TestRunLoginFilterAndPackets(t *testing.T) {
	loginCh := make(chan string, 1)
	addr := fakeServer(t, func(n int, login string, c net.Conn) {
		loginCh <- login
		c.Write([]byte("# keepalive comment\r\n"))
		c.Write([]byte("N0CALL>APRS,qAR,W0MIK-10:>one\r\n"))
		c.Write([]byte("N0CALL>APRS,qAR,W0MIK-10:>two\r\n"))
		time.Sleep(time.Second)
	})
	rec := newRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, testConfig(addr), rec.handler()) }()

	rec.waitPackets(t, 2)
	login := <-loginCh
	for _, want := range []string{"user W0MON", "pass -1", "filter e/W0MIK-10"} {
		if !strings.Contains(login, want) {
			t.Errorf("login line %q missing %q", login, want)
		}
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.packets) != 2 || strings.HasPrefix(rec.packets[0], "#") {
		t.Errorf("packets = %q, want 2 non-comment packets", rec.packets)
	}
}

func TestRunReconnectsAfterDrop(t *testing.T) {
	addr := fakeServer(t, func(n int, login string, c net.Conn) {
		c.Write([]byte("N0CALL>APRS,qAR,W0MIK-10:>hello\r\n"))
		if n == 0 {
			return // drop the first connection
		}
		time.Sleep(time.Second)
	})
	rec := newRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Run(ctx, testConfig(addr), rec.handler())

	rec.waitPackets(t, 2) // one from each connection
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.connects < 2 {
		t.Errorf("OnConnect called %d times, want >= 2", rec.connects)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	addr := fakeServer(t, func(n int, login string, c net.Conn) { time.Sleep(10 * time.Second) })
	rec := newRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, testConfig(addr), rec.handler()) }()
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestRunKeepsRetryingWhenServerDown(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close() // nothing listens here
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	err := Run(ctx, testConfig(addr), Handler{})
	if err != context.DeadlineExceeded {
		t.Errorf("Run returned %v, want DeadlineExceeded (kept retrying)", err)
	}
}
