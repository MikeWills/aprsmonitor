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
