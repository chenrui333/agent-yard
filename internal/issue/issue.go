package issue

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/chenrui333/agent-yard/internal/task"
)

type Checkbox struct {
	Text    string
	Checked bool
	Section string
	Slug    string
}

type ImportOptions struct {
	IssueNumber  int
	Limit        int
	Section      string
	IDPrefix     string
	BranchPrefix string
}

type ImportResult struct {
	Tasks   []task.Task
	Added   int
	Skipped int
}

var checkboxRE = regexp.MustCompile(`^\s*[-*+]\s+\[([ xX])\]\s+(.+?)\s*$`)
var headingRE = regexp.MustCompile(`^\s{0,3}(#{1,6})\s+(.+?)\s*#*\s*$`)

func ParseCheckboxes(body string) []Checkbox {
	var out []Checkbox
	section := ""
	for _, line := range strings.Split(body, "\n") {
		if matches := headingRE.FindStringSubmatch(line); len(matches) == 3 {
			section = cleanInlineMarkdown(matches[2])
			continue
		}
		matches := checkboxRE.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		text := cleanInlineMarkdown(matches[2])
		if text == "" {
			continue
		}
		out = append(out, Checkbox{
			Text:    text,
			Checked: strings.EqualFold(matches[1], "x"),
			Section: section,
			Slug:    Slug(text),
		})
	}
	return out
}

func ImportTasks(existing task.Ledger, boxes []Checkbox, opts ImportOptions) ImportResult {
	sectionFilter := Slug(opts.Section)
	seenIDs := map[string]bool{}
	seenIssueCheckbox := map[string]bool{}
	seenBranches := mapFromBranches(existing.Tasks, nil)
	result := ImportResult{}
	for _, item := range existing.Tasks {
		seenIDs[item.ID] = true
		if item.Issue == opts.IssueNumber && strings.TrimSpace(item.Checkbox) != "" {
			seenIssueCheckbox[checkboxKey(item.Checkbox)] = true
		}
	}

	for _, box := range boxes {
		if sectionFilter != "" && Slug(box.Section) != sectionFilter {
			continue
		}
		if box.Checked {
			result.Skipped++
			continue
		}
		if seenIssueCheckbox[checkboxKey(box.Text)] {
			result.Skipped++
			continue
		}
		if opts.Limit > 0 && result.Added >= opts.Limit {
			continue
		}
		id := uniqueID(defaultID(opts, box), seenIDs)
		seenIDs[id] = true
		seenIssueCheckbox[checkboxKey(box.Text)] = true
		branchBase := id
		if opts.BranchPrefix != "" {
			branchBase = opts.BranchPrefix + box.Slug
		}
		branch := uniqueID(branchBase, seenBranches)
		seenBranches[branch] = true
		result.Tasks = append(result.Tasks, task.Task{
			ID:            id,
			Issue:         opts.IssueNumber,
			Checkbox:      box.Text,
			ServiceFamily: Slug(box.Section),
			Branch:        branch,
			Status:        task.StatusReady,
			PRURL:         "",
			PRNumber:      0,
		})
		result.Added++
	}
	return result
}

func Slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func defaultID(opts ImportOptions, box Checkbox) string {
	prefix := opts.IDPrefix
	if prefix == "" {
		prefix = "issue-" + strconv.Itoa(opts.IssueNumber) + "-"
	}
	return prefix + box.Slug
}

func cleanInlineMarkdown(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`*_")
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func checkboxKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func uniqueID(base string, used map[string]bool) string {
	if base == "" {
		base = "task"
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := base + "-" + strconv.Itoa(i)
		if !used[candidate] {
			return candidate
		}
	}
}

func mapFromBranches(existing []task.Task, added []task.Task) map[string]bool {
	out := map[string]bool{}
	for _, item := range existing {
		if item.Branch != "" {
			out[item.Branch] = true
		}
	}
	for _, item := range added {
		if item.Branch != "" {
			out[item.Branch] = true
		}
	}
	return out
}
