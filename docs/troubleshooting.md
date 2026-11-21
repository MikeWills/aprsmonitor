# Troubleshooting and limitations

## Limitations (read these)

- **Quiet is indistinguishable from broken.** The monitor sees only what your
  iGate forwards. If the iGate is healthy but hears nothing for longer than
  `silence_threshold`, you get a false alarm. Tune the threshold to your
  station's quietest normal stretch.
- **It proves forwarding, not coverage.** A deaf receiver that still hears
  occasional strong packets keeps the monitor quiet.
- **Down alerts are sent once per outage.** If every channel fails all 3
  attempts, the alert is lost (it is logged as an error). Configure two
  channels, ideally independent ones (email *and* chat).
- **The monitor can be blind.** If AprsMonitor loses its own connection to
  APRS-IS it cannot judge the iGate and stays silent until it reconnects. It
  does not alert you about its own outage; run it under a supervisor that
  restarts it (systemd `Restart=always`) and consider an external uptime check.
- **Parser maturity.** The go-aprs-fap library says it is new and lightly
  tested. Gating is read from the packet header with a fallback that does not
  need the parser to understand the body, which limits the exposure.

## Common problems

**`config error: unknown setting`**: a key is misspelled; see the
[reference](configuration.md).

**Never sees any packets / always alerts.** Check on aprs.fi that your iGate
callsign and SSID are exactly what you put in `callsign`. The filter is
`e/<callsign>`, so `W0MIK-10` and `W0MIK-1` are different stations. Use a
server port that accepts filters (`14580`); the full-feed port `10152` ignores
them.

**`-test-alert` fails for email.** The error names the step. Common causes:
the server needs `security = "tls"` on port 465; STARTTLS not offered on the
port; app-specific password required (Gmail, Microsoft 365).

**Webhook returns 4xx.** The URL was revoked or mistyped. Create a new webhook.

**Log shows `aprs-is disconnected` repeatedly.** The monitor retries with
backoff (5 s up to 5 min). Persistent failures usually mean outbound TCP 14580
is blocked; try another network.
