package ssh

import (
	"reflect"
	"testing"
)

func TestSplitLogLines(t *testing.T) {
	out := "==> shop-main <==\r\nstarting up\nlistening on :3000\n"
	got := splitLogLines(out)
	want := []string{"==> shop-main <==", "starting up", "listening on :3000"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitLogLines = %v, want %v", got, want)
	}
}

func TestSplitLogLinesEmpty(t *testing.T) {
	if got := splitLogLines(""); len(got) != 0 {
		t.Errorf("splitLogLines(\"\") = %v, want empty", got)
	}
	if got := splitLogLines("\n"); len(got) != 0 {
		t.Errorf("splitLogLines(newline) = %v, want empty", got)
	}
}
