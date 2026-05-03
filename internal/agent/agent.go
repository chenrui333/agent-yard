package agent

import (
	"strings"
	"unicode"

	"github.com/chenrui333/agent-yard/internal/config"
	"github.com/chenrui333/agent-yard/internal/task"
)

func ShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func BuildLaunchCommand(worktree, promptFile string, command config.AgentCommand) string {
	parts := []string{ShellQuote(command.Command)}
	for _, arg := range command.Args {
		parts = append(parts, ShellQuote(arg))
	}
	return "cd " + ShellQuote(worktree) + " && " + strings.Join(parts, " ") + " < " + ShellQuote(promptFile)
}

func TaskWindowName(item task.Task) string {
	if item.AssignedAgent != "" {
		return SanitizeWindowName(item.AssignedAgent)
	}
	return SanitizeWindowName(item.ID)
}

func ReviewWindowName(prefix, id string) string {
	return SanitizeWindowName(prefix + "-" + id)
}

func SanitizeWindowName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "yard"
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range name {
		valid := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.'
		if valid {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-.")
	if result == "" {
		return "yard"
	}
	return result
}
