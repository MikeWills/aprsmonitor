package notify

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var msg = Message{Event: "down", Callsign: "W0MIK-10", Subject: "iGate W0MIK-10 is DOWN", Body: "Not heard for 30m0s."}

func TestWebhookPostsJSONForDiscordAndSlack(t *testing.T) {
	var got map[string]string
	var ctype string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctype = r.Header.Get("Content-Type")
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusNoContent) // what Discord returns
	}))
	defer srv.Close()

	if err := NewWebhook(srv.URL).Send(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if ctype != "application/json" {
		t.Errorf("content-type = %q", ctype)
	}
	for _, k := range []string{"content", "text"} { // Discord reads content, Slack reads text
		if !strings.Contains(got[k], "DOWN") || !strings.Contains(got[k], "30m0s") {
			t.Errorf("%s = %q, want subject and body", k, got[k])
		}
	}
	if got["event"] != "down" || got["callsign"] != "W0MIK-10" {
		t.Errorf("generic fields wrong: %v", got)
	}
}

func TestWebhookErrorOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadRequest)
	}))
	defer srv.Close()
	if err := NewWebhook(srv.URL).Send(context.Background(), msg); err == nil {
		t.Fatal("want error for HTTP 400")
	}
}

type flaky struct {
	failures int32
	calls    atomic.Int32
}

func (f *flaky) Name() string { return "flaky" }
func (f *flaky) Send(context.Context, Message) error {
	if f.calls.Add(1) <= f.failures {
		return errors.New("boom")
	}
	return nil
}

func TestDispatchRetriesAndReachesAllNotifiers(t *testing.T) {
	recovers := &flaky{failures: 2}
	broken := &flaky{failures: 100}
	healthy := &flaky{}
	err := Dispatch(context.Background(), []Notifier{broken, recovers, healthy}, msg, 3, time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "flaky") {
		t.Fatalf("err = %v, want error naming the broken notifier", err)
	}
	if recovers.calls.Load() != 3 {
		t.Errorf("recovering notifier tried %d times, want 3", recovers.calls.Load())
	}
	if broken.calls.Load() != 3 {
		t.Errorf("broken notifier tried %d times, want 3 (bounded)", broken.calls.Load())
	}
	if healthy.calls.Load() != 1 {
		t.Errorf("healthy notifier called %d times, want 1", healthy.calls.Load())
	}
}

func TestDispatchNoErrorWhenAllSucceed(t *testing.T) {
	if err := Dispatch(context.Background(), []Notifier{&flaky{}}, msg, 3, time.Millisecond); err != nil {
		t.Fatal(err)
	}
}

// fakeSMTP implements just enough SMTP to capture one message.
func fakeSMTP(t *testing.T) (addr string, captured chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	captured = make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		say := func(s string) { c.Write([]byte(s + "\r\n")) }
		say("220 fake ESMTP")
		var sb strings.Builder
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if inData {
				if line == "." {
					inData = false
					captured <- sb.String()
					say("250 queued")
					continue
				}
				sb.WriteString(line + "\n")
				continue
			}
			switch cmd := strings.ToUpper(line); {
			case strings.HasPrefix(cmd, "EHLO"):
				say("250 fake")
			case strings.HasPrefix(cmd, "MAIL FROM"), strings.HasPrefix(cmd, "RCPT TO"):
				sb.WriteString(line + "\n")
				say("250 ok")
			case cmd == "DATA":
				say("354 go")
				inData = true
			case cmd == "QUIT":
				say("221 bye")
				return
			default:
				say("250 ok")
			}
		}
	}()
	return ln.Addr().String(), captured
}

func TestEmailSendsMessage(t *testing.T) {
	addr, captured := fakeSMTP(t)
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)
	e := &Email{Host: host, Port: port, Security: "none",
		From: "aprsmonitor@example.org", To: []string{"mike@example.org", "ops@example.org"}}

	m := msg
	m.Subject = "iGate W0MIK-10 is DOWN\r\nBcc: evil@example.org" // header injection attempt
	if err := e.Send(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-captured:
		for _, want := range []string{"MAIL FROM:<aprsmonitor@example.org>", "RCPT TO:<mike@example.org>",
			"RCPT TO:<ops@example.org>", "Subject: iGate W0MIK-10 is DOWN", "Not heard for 30m0s."} {
			if !strings.Contains(got, want) {
				t.Errorf("captured message missing %q:\n%s", want, got)
			}
		}
		if strings.Contains(got, "\nBcc:") {
			t.Errorf("header injection succeeded:\n%s", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no message captured")
	}
}
