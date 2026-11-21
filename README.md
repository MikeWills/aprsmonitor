# AprsMonitor

Get an email or chat message when your APRS iGate stops working.

AprsMonitor connects to APRS-IS as a read-only client and watches for packets
that your iGate forwards. If none arrive for a set time, it alerts you once.
When the iGate is heard again it tells you that too.

- Single static binary for Windows, Linux, macOS and Raspberry Pi
- Alerts by **email (SMTP)** and **webhook** (Discord, Slack, Mattermost, generic)
- Run it anywhere with internet access, ideally *not* on the iGate itself
  (if that machine dies, so does the monitor)
- Uses the [go-aprs-fap](https://github.com/hessu/go-aprs-fap) APRS parser

## Quick start

1. Install [Go](https://go.dev/dl/) and build:

   ```
   go build -o aprsmonitor ./cmd/aprsmonitor
   ```

   or `go install github.com/MikeWills/aprsmonitor/cmd/aprsmonitor@latest`.

2. Copy `config.example.toml` to `config.toml`, set your iGate callsign
   (with SSID) and add an `[email]` or `[webhook]` section.
3. Check it and send yourself a test alert:

   ```
   ./aprsmonitor -check-config
   ./aprsmonitor -test-alert
   ```

4. Run it: `./aprsmonitor` (see [deployment](docs/deployment.md) to keep it
   running as a service).

## Documentation

- [Configuration reference](docs/configuration.md)
- [Deployment: systemd, Windows, Docker](docs/deployment.md)
- [Design and rationale](docs/design.md)
- [Troubleshooting and limitations](docs/troubleshooting.md)

## Development

```
go test ./...
```

The tests need no network: APRS-IS and SMTP are faked in-process.

## License

MIT, see [LICENSE](LICENSE). The go-aprs-fap dependency is BSD 3-clause.
