# Configuration reference

Config is a TOML file, `config.toml` in the working directory by default
(`-config path` to change). Unknown keys are rejected so typos are caught at
startup. Validate with `aprsmonitor -check-config`.

## Top level

| Key | Default | Meaning |
|---|---|---|
| `callsign` | required | iGate to watch, with SSID, e.g. `W0MIK-10`. SSIDs are exact. |
| `silence_threshold` | `"1h"` | Alert after this long with no gated packet. Go duration (`"45m"`, `"2h"`). Minimum `1m`. |
| `count_own_packets` | `false` | Also count packets whose source is the iGate itself. |
| `notify_recovery` | `true` | Send a message when the iGate is heard again. |

**Choosing `silence_threshold`.** The monitor can only see traffic your iGate
forwards. A busy urban iGate hears something every few minutes, so `15m`-`30m`
is safe. A rural iGate on a quiet frequency may go hours with nothing to
forward; set the threshold above your longest normal gap. Look at your
station's history on aprs.fi to pick a value. Too low means false alarms.

**`count_own_packets`.** If your iGate also beacons its own position over the
internet, those packets show it has a network link, but not that its radio
receiver works, so they are ignored unless you turn this on.

## `[aprsis]`

| Key | Default | Meaning |
|---|---|---|
| `server` | `rotate.aprs2.net:14580` | APRS-IS server. Must be a port that accepts user filters (14580). |
| `login_callsign` | same as `callsign` | Callsign used to log in. |
| `passcode` | `"-1"` | `-1` is a read-only login; no passcode needed. |

## `[email]`

Used when any of `host`, `from`, `to` is set; then `host`, `from`, `to` are all required.

| Key | Default | Meaning |
|---|---|---|
| `host` | | SMTP server |
| `port` | `587` | |
| `security` | `"starttls"` | `starttls`, `tls` (implicit TLS, port 465) or `none` (trusted local relay only) |
| `username`, `password` | empty | SMTP login; omit both for no authentication |
| `from` | | Sender address |
| `to` | | List of recipient addresses |

`starttls` refuses to send if the server does not offer it; it never silently
falls back to plain text.

## `[webhook]`

| Key | Meaning |
|---|---|
| `url` | `http(s)` endpoint. The JSON body has `content` (Discord), `text` (Slack, Mattermost), and `event`, `callsign`, `subject`, `body`. |

For Discord create a webhook in channel settings, Integrations. For Slack use an
incoming webhook URL.

## Keeping secrets out of the file

Any of `email.username`, `email.password`, `webhook.url` and
`aprsis.passcode` can be written as `env:NAME` to read environment variable
`NAME` at startup:

```toml
password = "env:APRSMON_SMTP_PASSWORD"
```

A missing or empty variable is a startup error. Webhook URLs contain a secret
token; AprsMonitor removes them from error messages and logs.

## Command-line flags

| Flag | Meaning |
|---|---|
| `-config path` | Config file (default `config.toml`) |
| `-check-config` | Validate and exit |
| `-test-alert` | Send a test message through every channel and exit |
| `-version` | Print version |
