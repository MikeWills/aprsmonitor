// Package aprsis keeps a read-only, filtered connection to an APRS-IS server
// alive and delivers packets to a handler.
package aprsis

import (
	"context"
	"errors"
	"log/slog"
	"time"

	fap "github.com/hessu/go-aprs-fap"
)

// Config describes the APRS-IS connection.
type Config struct {
	Server     string // host:port, e.g. "rotate.aprs2.net:14580"
	Callsign   string // login callsign (the monitor's own, not necessarily the iGate)
	Passcode   string // "-1" for read-only
	Filter     string // server-side filter, e.g. "e/W0MIK-10"
	AppVersion string

	// ReadTimeout is how long one read may block. Servers send a keepalive
	// comment about every 20s, so silence beyond this means a dead link.
	ReadTimeout time.Duration
	// MinBackoff and MaxBackoff bound the delay between reconnect attempts.
	MinBackoff, MaxBackoff time.Duration

	Logger *slog.Logger
}

// Handler receives connection events. Any field may be nil. Callbacks run on
// the connection goroutine and must not block for long.
type Handler struct {
	OnConnect    func()
	OnDisconnect func(err error)
	OnPacket     func(raw string)
}

const dialTimeout = 15 * time.Second

// Run connects, delivers packets, and reconnects with exponential backoff
// until ctx is cancelled. It returns ctx.Err().
func Run(ctx context.Context, cfg Config, h Handler) error {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 90 * time.Second
	}
	if cfg.MinBackoff <= 0 {
		cfg.MinBackoff = 5 * time.Second
	}
	if cfg.MaxBackoff < cfg.MinBackoff {
		cfg.MaxBackoff = 5 * time.Minute
	}

	backoff := cfg.MinBackoff
	for ctx.Err() == nil {
		connected, err := session(ctx, cfg, h)
		if ctx.Err() != nil {
			break
		}
		if connected {
			backoff = cfg.MinBackoff // a working link resets the backoff
		}
		cfg.Logger.Warn("aprs-is disconnected", "err", err, "retry_in", backoff)
		if h.OnDisconnect != nil {
			h.OnDisconnect(err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(backoff):
		}
		if backoff *= 2; backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}
	}
	return ctx.Err()
}

// session runs one connection until it fails. connected reports whether the
// login succeeded.
func session(ctx context.Context, cfg Config, h Handler) (connected bool, err error) {
	conn, err := fap.Connect(cfg.Server, fap.ConnOptions{
		Callsign:     cfg.Callsign,
		Passcode:     cfg.Passcode,
		AppName:      "aprsmonitor",
		AppVer:       cfg.AppVersion,
		Filter:       cfg.Filter,
		DialTimeout:  dialTimeout,
		LoginTimeout: dialTimeout,
		// Plain text keeps behaviour identical on every server.
		NoIS2Upgrade: true,
	})
	if err != nil {
		return false, err
	}
	defer conn.Close()

	// Unblock a pending read when the context is cancelled.
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	cfg.Logger.Info("aprs-is connected", "server", cfg.Server, "filter", cfg.Filter)
	if h.OnConnect != nil {
		h.OnConnect()
	}
	for {
		raw, err := conn.ReadPacket(cfg.ReadTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return true, ctx.Err()
			}
			return true, errors.Join(errors.New("read"), err)
		}
		if h.OnPacket != nil {
			h.OnPacket(raw)
		}
	}
}
