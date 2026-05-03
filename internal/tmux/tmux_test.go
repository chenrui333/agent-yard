package tmux

import "testing"

func TestParsePaneList(t *testing.T) {
	got := ParsePaneList("%1\tcodex\t0\t\n%2\tzsh\t1\t2\n")
	if len(got) != 2 {
		t.Fatalf("len = %d; want 2", len(got))
	}
	if got[0].ID != "%1" || got[0].CurrentCommand != "codex" || got[0].Dead {
		t.Fatalf("first pane = %#v", got[0])
	}
	if got[1].ID != "%2" || !got[1].Dead || got[1].DeadStatus != "2" {
		t.Fatalf("second pane = %#v", got[1])
	}
}
