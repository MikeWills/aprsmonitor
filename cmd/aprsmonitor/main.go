// Command aprsmonitor alerts you when your APRS iGate stops gating packets.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MikeWills/aprsmonitor/internal/aprsis"
	"github.com/MikeWills/aprsmonitor/internal/config"
	"github.com/MikeWills/aprsmonitor/internal/monitor"
	"github.com/MikeWills/aprsmonitor/internal/notify"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const tickInterval = 30 * time.Second

func main() {
	os.Exit(run())
}

func run() int {
	var (
		cfgPath   = flag.String("config", "config.toml", "path to the TOML configuration file")
		checkOnly = flag.Bool("check-config", false, "validate the configuration and exit")
		testAlert = flag.Bool("test-alert", false, "send a test notification through every channel and exit")
		showVer   = flag.Bool("version", false, "print the version and exit")
	)
	flag.Parse()
	if *showVer {
		fmt.Println("aprsmonitor", version)
		return 0
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Error("configuration error", "err", err)
		return 2
	}
	if *checkOnly {
		fmt.Printf("config OK: watching %s, alert after %s of silence, filter %q\n",
			cfg.Callsign, cfg.SilenceThreshold.D, cfg.Filter())
		return 0
	}

	mon := monitor.New(monitor.Config{
		Callsign:        cfg.Callsign,
		Threshold:       cfg.SilenceThreshold.D,
		CountOwnPackets: cfg.CountOwnPackets,
		NotifyRecovery:  cfg.NotifyRecovery,
		Notifiers:       notifiers(cfg),
		Logger:          log,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *testAlert {
		if err := mon.SendTest(ctx); err != nil {
			log.Error("test alert failed", "err", err)
			return 1
		}
		fmt.Println("test alert sent")
		return 0
	}

	log.Info("starting", "version", version, "callsign", cfg.Callsign,
		"threshold", cfg.SilenceThreshold.D, "filter", cfg.Filter())
	go mon.Run(ctx, tickInterval)
	aprsis.Run(ctx, aprsis.Config{
		Server:     cfg.APRSIS.Server,
		Callsign:   cfg.APRSIS.LoginCallsign,
		Passcode:   cfg.APRSIS.Passcode,
		Filter:     cfg.Filter(),
		AppVersion: version,
		Logger:     log,
	}, aprsis.Handler{
		OnConnect:    mon.OnConnect,
		OnDisconnect: mon.OnDisconnect,
		OnPacket:     mon.OnPacket,
	})
	log.Info("stopped")
	return 0
}

func notifiers(cfg *config.Config) []notify.Notifier {
	var ns []notify.Notifier
	if cfg.EmailEnabled() {
		ns = append(ns, &notify.Email{
			Host: cfg.Email.Host, Port: cfg.Email.Port,
			Username: cfg.Email.Username, Password: cfg.Email.Password,
			From: cfg.Email.From, To: cfg.Email.To, Security: cfg.Email.Security,
		})
	}
	if cfg.WebhookEnabled() {
		ns = append(ns, notify.NewWebhook(cfg.Webhook.URL))
	}
	return ns
}
