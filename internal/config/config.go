// Package config loads and validates the TOML configuration file.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Duration is a time.Duration that reads Go duration strings ("45m", "2h").
type Duration struct{ D time.Duration }

func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	d.D = v
	return nil
}

type APRSIS struct {
	Server        string `toml:"server"`
	LoginCallsign string `toml:"login_callsign"`
	Passcode      string `toml:"passcode"`
}

type Email struct {
	Host     string   `toml:"host"`
	Port     int      `toml:"port"`
	Username string   `toml:"username"`
	Password string   `toml:"password"`
	From     string   `toml:"from"`
	To       []string `toml:"to"`
	Security string   `toml:"security"`
}

type Webhook struct {
	URL string `toml:"url"`
}

// Config is the whole configuration.
type Config struct {
	Callsign         string   `toml:"callsign"`
	SilenceThreshold Duration `toml:"silence_threshold"`
	CountOwnPackets  bool     `toml:"count_own_packets"`
	NotifyRecovery   bool     `toml:"notify_recovery"`
	APRSIS           APRSIS   `toml:"aprsis"`
	Email            Email    `toml:"email"`
	Webhook          Webhook  `toml:"webhook"`
}

// MinThreshold is the smallest allowed silence_threshold. Below this, normal
// gaps in RF traffic would cause false alarms.
const MinThreshold = time.Minute

var callRe = regexp.MustCompile(`^[A-Z0-9]{3,9}(-[0-9]{1,2})?$`)

// EmailEnabled reports whether the [email] section is in use.
func (c *Config) EmailEnabled() bool {
	return c.Email.Host != "" || c.Email.From != "" || len(c.Email.To) > 0
}

// WebhookEnabled reports whether the [webhook] section is in use.
func (c *Config) WebhookEnabled() bool { return c.Webhook.URL != "" }

// Filter returns the APRS-IS server-side filter for the watched station.
func (c *Config) Filter() string {
	f := "e/" + c.Callsign
	if c.CountOwnPackets {
		f += " b/" + c.Callsign
	}
	return f
}

// Load reads, defaults and validates the file at path.
func Load(path string) (*Config, error) {
	c := &Config{
		SilenceThreshold: Duration{time.Hour},
		NotifyRecovery:   true,
		APRSIS:           APRSIS{Server: "rotate.aprs2.net:14580", Passcode: "-1"},
		Email:            Email{Port: 587, Security: "starttls"},
	}
	md, err := toml.DecodeFile(path, c)
	if err != nil {
		var pe toml.ParseError
		if errors.As(err, &pe) {
			return nil, fmt.Errorf("%s: %s", path, pe.ErrorWithPosition())
		}
		// Duration errors name the key via the toml decoder context.
		return nil, fmt.Errorf("%s: %w (check silence_threshold uses a form like \"45m\" or \"2h\")", path, err)
	}
	if u := md.Undecoded(); len(u) > 0 {
		return nil, fmt.Errorf("%s: unknown setting %q (typo?)", path, u[0].String())
	}
	if err := c.finish(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

func (c *Config) finish() error {
	c.Callsign = strings.ToUpper(strings.TrimSpace(c.Callsign))
	if !callRe.MatchString(c.Callsign) {
		return fmt.Errorf("callsign %q is missing or invalid (expected like W0MIK-10)", c.Callsign)
	}
	if c.SilenceThreshold.D < MinThreshold {
		return fmt.Errorf("silence_threshold %v is too small (minimum %v)", c.SilenceThreshold.D, MinThreshold)
	}
	if c.APRSIS.LoginCallsign == "" {
		c.APRSIS.LoginCallsign = c.Callsign
	}
	c.APRSIS.LoginCallsign = strings.ToUpper(c.APRSIS.LoginCallsign)

	var err error
	if c.Email.Username, err = resolve(c.Email.Username); err != nil {
		return fmt.Errorf("email.username: %w", err)
	}
	if c.Email.Password, err = resolve(c.Email.Password); err != nil {
		return fmt.Errorf("email.password: %w", err)
	}
	if c.Webhook.URL, err = resolve(c.Webhook.URL); err != nil {
		return fmt.Errorf("webhook.url: %w", err)
	}
	if c.APRSIS.Passcode, err = resolve(c.APRSIS.Passcode); err != nil {
		return fmt.Errorf("aprsis.passcode: %w", err)
	}

	if !c.EmailEnabled() && !c.WebhookEnabled() {
		return errors.New("no notification channel configured: add an [email] or [webhook] section")
	}
	if c.EmailEnabled() {
		e := &c.Email
		if e.Host == "" || e.From == "" || len(e.To) == 0 {
			return errors.New("email needs host, from and to")
		}
		e.Security = strings.ToLower(e.Security)
		switch e.Security {
		case "starttls", "tls", "none":
		default:
			return fmt.Errorf("email.security %q must be starttls, tls or none", e.Security)
		}
	}
	if c.WebhookEnabled() {
		u, err := url.Parse(c.Webhook.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("webhook.url must be an http(s) URL")
		}
	}
	return nil
}

// resolve returns the value of an "env:NAME" reference, or s unchanged. Using
// an explicit prefix (instead of $VAR expansion) keeps passwords containing
// "$" intact.
func resolve(s string) (string, error) {
	name, ok := strings.CutPrefix(s, "env:")
	if !ok {
		return s, nil
	}
	v, set := os.LookupEnv(name)
	if !set || v == "" {
		return "", fmt.Errorf("environment variable %s is not set", name)
	}
	return v, nil
}
