package gate

import "testing"

func TestHeard(t *testing.T) {
	const ig = "W0MIK-10"
	tests := []struct {
		name       string
		raw        string
		includeOwn bool
		want       bool
	}{
		{"gated via qAR", "N0CALL>APRS,WIDE1-1,qAR,W0MIK-10:!6128.23N/02353.52E-PHG2360/Testing", false, true},
		{"gated via qAO", "N0CALL>APRS,qAO,W0MIK-10:>status text", false, true},
		{"lowercase call in path", "N0CALL>APRS,qAR,w0mik-10:>status", false, true},
		{"gated by someone else", "N0CALL>APRS,WIDE1-1,qAR,K0ABC-5:>status", false, false},
		{"call only as digipeater before q", "N0CALL>APRS,W0MIK-10*,WIDE2-1,qAR,K0ABC-5:>status", false, false},
		{"own packet from server, option off", "W0MIK-10>APRS,TCPIP*,qAC,T2SERVER:>status", false, false},
		{"own packet from server, option on", "W0MIK-10>APRS,TCPIP*,qAC,T2SERVER:>status", true, true},
		{"unparseable body still counts", "N0CALL>APRS,qAR,W0MIK-10:\x00\x01 not aprs", false, true},
		{"different ssid is a different station", "N0CALL>APRS,qAR,W0MIK-1:>status", false, false},
		{"garbage", "this is not a packet", false, false},
		{"empty", "", false, false},
		{"comment line", "# aprsc 2.1.19 26 Nov 2026 19:00:00 GMT T2SERVER 1.2.3.4:14580", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := New(ig, tc.includeOwn)
			if got := m.Heard(tc.raw); got != tc.want {
				t.Errorf("Heard(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// fap rejects 7-character callsigns; the header fallback must still match.
func TestHeardFallbackForNonStandardCall(t *testing.T) {
	m := New("MYIGATE-10", false)
	if !m.Heard("N0CALL>APRS,qAR,MYIGATE-10:>x") {
		t.Error("non-standard callsign in path should match via fallback")
	}
}
