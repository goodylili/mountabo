package ssh

import (
	"reflect"
	"testing"
)

func TestParseListeningPorts(t *testing.T) {
	// Representative `ss -H -ltun | awk '{print $5}'` output: IPv4, IPv6, loopback,
	// a duplicate (same port on two interfaces), and junk lines to ignore.
	out := `0.0.0.0:22
[::]:22
127.0.0.1:323
0.0.0.0:80
[::]:443

*:*
not-a-port
`
	got := parseListeningPorts(out)
	want := []int{22, 80, 323, 443}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseListeningPorts = %v, want %v", got, want)
	}
}
