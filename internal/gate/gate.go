// Package gate decides whether an APRS-IS packet proves the watched iGate is
// forwarding traffic.
package gate

import (
	"strings"

	fap "github.com/hessu/go-aprs-fap"
)

// Matcher recognises packets gated by one station.
type Matcher struct {
	call       string // upper-case, exact SSID
	includeOwn bool
}

// New returns a Matcher for callsign (with SSID, e.g. "N0CALL-10"). If
// includeOwn is true, packets whose source is the callsign itself also count.
func New(callsign string, includeOwn bool) *Matcher {
	return &Matcher{call: strings.ToUpper(strings.TrimSpace(callsign)), includeOwn: includeOwn}
}

// Heard reports whether raw (one APRS-IS line) was gated by the station: its
// path has a q construct (qAR, qAO, ...) immediately followed by the callsign.
func (m *Matcher) Heard(raw string) bool {
	if m.call == "" || raw == "" || strings.HasPrefix(raw, "#") {
		return false
	}
	src, path, ok := splitHeader(raw)
	if !ok {
		return false
	}
	if m.includeOwn && src == m.call {
		return true
	}
	for i := 0; i+1 < len(path); i++ {
		if len(path[i]) == 3 && path[i][0] == 'Q' && path[i+1] == m.call {
			return true
		}
	}
	return false
}

// splitHeader returns the upper-case source callsign and path elements.
//
// It uses the fap parser first. If fap rejects the packet (for example a
// packet type it does not implement, or a malformed body) it falls back to
// splitting the header by hand: the gating information is in the header, so a
// packet we cannot fully decode must still count as proof of life, otherwise
// an exotic packet type could cause a false "iGate down" alert.
func splitHeader(raw string) (src string, path []string, ok bool) {
	if p, err := fap.Parse(raw); err == nil && p.SrcCallsign != "" {
		path = make([]string, 0, len(p.Digipeaters))
		for _, d := range p.Digipeaters {
			path = append(path, strings.ToUpper(d.Call))
		}
		return strings.ToUpper(p.SrcCallsign), path, true
	}

	header, _, found := strings.Cut(raw, ":")
	if !found {
		return "", nil, false
	}
	src, rest, found := strings.Cut(header, ">")
	if !found || src == "" {
		return "", nil, false
	}
	elems := strings.Split(rest, ",") // dst, then path
	for _, e := range elems[1:] {
		path = append(path, strings.ToUpper(strings.TrimSuffix(e, "*")))
	}
	return strings.ToUpper(src), path, true
}
