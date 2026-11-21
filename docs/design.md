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
