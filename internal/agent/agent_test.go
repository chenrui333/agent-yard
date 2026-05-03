package agent

import (
	"testing"

	"github.com/chenrui333/agent-yard/internal/config"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "''"},
		{"simple", "'simple'"},
		{"has space", "'has space'"},
		{"it's", "'it'\\''s'"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ShellQuote(tt.input); got != tt.want {
				t.Fatalf("ShellQuote(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildLaunchCommand(t *testing.T) {
	cmd := BuildLaunchCommand("/tmp/work tree", "/tmp/prompt.md", config.AgentCommand{
		Command: "codex",
		Args:    []string{"exec", "--sandbox", "danger-full-access"},
	})
	want := "cd '/tmp/work tree' && 'codex' 'exec' '--sandbox' 'danger-full-access' < '/tmp/prompt.md'"
	if cmd != want {
		t.Fatalf("BuildLaunchCommand = %q; want %q", cmd, want)
	}
}

func TestSanitizeWindowName(t *testing.T) {
	if got := SanitizeWindowName("local review: route53"); got != "local-review-route53" {
		t.Fatalf("SanitizeWindowName = %q", got)
	}
}
