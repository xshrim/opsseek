package host

import "testing"

func TestParseListenerTableUsesOnlyProcessSocketInodes(t *testing.T) {
	raw := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  100        0 42 1 0000000000000000 100 0 0 10 0
   1: 00000000:2328 00000000:0000 0A 00000000:00000000 00:00000000 00000000  100        0 43 1 0000000000000000 100 0 0 10 0`
	listeners := parseListenerTable(raw, map[string]struct{}{"42": {}}, "tcp", "0A")
	if len(listeners) != 1 || listeners[0].Port != 8080 || listeners[0].Protocol != "tcp" {
		t.Fatalf("listeners = %#v, want tcp/8080", listeners)
	}
}
