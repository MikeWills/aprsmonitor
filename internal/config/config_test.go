package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const minimal = `
callsign = "W0MIK-10"
[webhook]
url = "https://discord.com/api/webhooks/1/abc"
`

func TestLoadDefaults(t *testing.T) {
	c, err := Load(write(t, minimal))
	if err != nil {
		t.Fatal(err)
	}
	if c.Callsign != "W0MIK-10" {
		t.Errorf("callsign = %q", c.Callsign)
	}
	if c.SilenceThreshold.D != time.Hour {
		t.Errorf("default threshold = %v, want 1h", c.SilenceThreshold.D)
	}
	if c.APRSIS.Server != "rotate.aprs2.net:14580" || c.APRSIS.Passcode != "-1" {
		t.Errorf("aprs-is defaults wrong: %+v", c.APRSIS)
	}
	if c.APRSIS.LoginCallsign != "W0MIK-10" {
		t.Errorf("login callsign should default to callsign, got %q", c.APRSIS.LoginCallsign)
	}
	if !c.NotifyRecovery {
		t.Error("notify_recovery should default to true")
	}
	if got := c.Filter(); got != "e/W0MIK-10" {
		t.Errorf("filter = %q", got)
	}
}

func TestFilterIncludesOwnPackets(t *testing.T) {
	c, err := Load(write(t, minimal+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	c.CountOwnPackets = true
	if got := c.Filter(); got != "e/W0MIK-10 b/W0MIK-10" {
		t.Errorf("filter = %q", got)
	}
}

func TestLoadFullEmailAndEnvSecrets(t *testing.T) {
	t.Setenv("TEST_SMTP_PASS", "s3cr$t")
	c, err := Load(write(t, `
callsign = "w0mik-10"
silence_threshold = "45m"
[email]
host = "smtp.example.org"
username = "bot"
password = "env:TEST_SMTP_PASS"
from = "aprsmonitor@example.org"
to = ["mike@example.org"]
`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Callsign != "W0MIK-10" {
		t.Errorf("callsign should be upper-cased, got %q", c.Callsign)
	}
	if c.SilenceThreshold.D != 45*time.Minute {
		t.Errorf("threshold = %v", c.SilenceThreshold.D)
	}
	if c.Email.Password != "s3cr$t" {
		t.Errorf("env: password not resolved, got %q", c.Email.Password)
	}
	if c.Email.Port != 587 || c.Email.Security != "starttls" {
		t.Errorf("email defaults wrong: %+v", c.Email)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct{ name, toml, wantErr string }{
		{"missing callsign", `[webhook]
url = "https://x.example/h"`, "callsign"},
		{"bad callsign", `callsign = "not a call!"
[webhook]
url = "https://x.example/h"`, "callsign"},
		{"no notifier", `callsign = "W0MIK-10"`, "notification"},
		{"threshold too small", `callsign = "W0MIK-10"
silence_threshold = "10s"
[webhook]
url = "https://x.example/h"`, "silence_threshold"},
		{"threshold unparsable", `callsign = "W0MIK-10"
silence_threshold = "soon"
[webhook]
url = "https://x.example/h"`, "silence_threshold"},
		{"webhook not http", `callsign = "W0MIK-10"
[webhook]
url = "ftp://x"`, "webhook"},
		{"email incomplete", `callsign = "W0MIK-10"
[email]
host = "smtp.example.org"`, "email"},
		{"email bad security", `callsign = "W0MIK-10"
[email]
host = "h"
from = "a@b.org"
to = ["c@d.org"]
security = "ssl3"`, "security"},
		{"unset env secret", `callsign = "W0MIK-10"
[webhook]
url = "env:DEFINITELY_NOT_SET_12345"`, "DEFINITELY_NOT_SET_12345"},
		{"unknown key (typo)", `callsign = "W0MIK-10"
silence_treshold = "1h"
[webhook]
url = "https://x.example/h"`, "silence_treshold"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(write(t, tc.toml))
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q should mention %q", err, tc.wantErr)
			}
		})
	}
}
