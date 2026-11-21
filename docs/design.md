# Design

AprsMonitor alerts the owner when their APRS iGate stops working.

## What "working" means

An iGate is working when it hears packets on RF and forwards them to APRS-IS.
APRS-IS tags every forwarded packet with a *q construct* followed by the
callsign of the station that injected it, for example:

    N0CALL>APRS,WIDE1-1,qAR,MYIGATE-10:!6128.23N/02353.52E-Testing

Here `MYIGATE-10` gated the packet. So the monitor connects to APRS-IS as a
read-only client and asks the server for packets entered by that callsign.
If none arrive for the configured time, the iGate is down (or has no RF
traffic, see Limitations).

## Silence tracker (`internal/watch`)

A small state machine, independent of the network, so it is easy to test.

- `Down` is reported once per outage, never repeated.
- `Recovered` is reported only after a packet is actually heard.
- A fresh start gets a full threshold of grace (no instant alert on launch).
- `Resume` is called on every APRS-IS (re)connect. Time the monitor spent
  disconnected is not evidence about the iGate, so the silence clock restarts.
  Reconnecting never changes Down/Recovered state by itself.

## Matching gated packets (`internal/gate`)

A packet counts as proof of life when its path has a q construct (`qAR`,
`qAO`, ...) immediately followed by the watched callsign. SSIDs are exact:
`W0MIK-1` and `W0MIK-10` are different stations. Optionally
(`count_own_packets`) packets *sourced* by the callsign count too.

Parsing uses `go-aprs-fap` first. Two things we verified about it:

- It accepts q constructs in the path (they appear as digipeaters).
- It rejects packets it cannot fully decode (unsupported packet types,
  callsigns longer than 6 characters, etc.).

The gating information lives in the header, not the body, so a packet we
cannot decode is still valid proof of life. On a parse error the matcher
splits the header by hand. This avoids a false "iGate down" alert caused by an
unusual packet type. The library's README warns it is new and lightly tested;
this fallback also limits our exposure to its bugs.

## APRS-IS connection (`internal/aprsis`)

`aprsis.Run` holds one read-only connection and reconnects with exponential
backoff (5 s doubling to 5 min; reset after any successful login).

- **Server-side filter.** The login sends `filter e/<CALL>`. The APRS-IS
  *entry station* filter passes exactly the packets whose q construct is
  followed by that callsign (checked against the APRS-IS filter docs), so the
  monitor receives only the iGate's own traffic, a trickle rather than the
  firehose. When `count_own_packets` is on, `b/<CALL>` is added.
- **Dead links.** Servers send a `#` keepalive about every 20 s. A read that
  returns nothing for `ReadTimeout` (90 s) is treated as a dead link and the
  client reconnects.
- **Plain text only.** IS2 upgrade is disabled so behaviour is identical on
  every server.
- **Tests** run against an in-process fake APRS-IS server (login line,
  filter, packets, drop and reconnect, cancel), so no network is needed.

## Notifications (`internal/notify`)

Two channels in v1, both behind one `Notifier` interface so more can be added
without touching the monitor loop.

- **Email**: SMTP with `starttls` (default, requires the server to offer it,
  never silently downgrades), implicit `tls`, or `none` for a trusted local
  relay. Subject lines have CR/LF stripped so event text can never inject
  headers; addresses are parsed, not concatenated.
- **Webhook**: JSON POST. The text is sent as both `content` (Discord) and
  `text` (Slack, Mattermost, Rocket.Chat), plus `event`, `callsign`,
  `subject`, `body` for generic receivers. Webhook URLs are secrets (the token
  is in the path), so they are stripped from error messages and logs.
- **Delivery**: `Dispatch` tries every notifier independently, retrying each
  up to 3 times with doubling delay. One broken channel never blocks another,
  and the error names every channel that gave up. A `Down` alert is sent once
  per outage, so retry matters: a lost alert is not re-sent.

## Monitor loop (`internal/monitor`)

Glue between the pieces. Packets arrive on the APRS-IS goroutine; a 30 s
ticker evaluates the tracker.

- **Blind is not down.** While the monitor itself is disconnected from
  APRS-IS it raises no alert, and on reconnect the silence clock restarts.
  Otherwise a network blip on the monitor's host would report a healthy iGate
  as dead.
- **Lock discipline.** State is guarded by a mutex, but notifications are sent
  after the lock is released, so slow SMTP never stalls packet intake.
- Alert text names the station, the threshold, when it was last heard, and
  what to check. `-test-alert` sends a test through every channel.

## Configuration (`internal/config`)

TOML, strict: unknown keys are errors (a typo like `silence_treshold` must not
silently fall back to the default). Secrets can be written as `env:NAME`
to read them from the environment instead of the file; an explicit prefix is
used instead of `$VAR` expansion so passwords containing `$` stay intact. An
unset variable is a startup error, not an empty password. See
[configuration.md](configuration.md).
