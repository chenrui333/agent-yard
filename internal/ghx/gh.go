package ghx

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/chenrui333/agent-yard/internal/execx"
)

type Runner interface {
	Run(context.Context, execx.Command) (execx.Result, error)
}

type Client struct {
	Runner Runner
}

type Issue struct {
	Body  string `json:"body"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type PullRequest struct {
	Number            int           `json:"number"`
	Title             string        `json:"title"`
	URL               string        `json:"url"`
	State             string        `json:"state"`
	HeadRefName       string        `json:"headRefName"`
	HeadRefOid        string        `json:"headRefOid"`
	BaseRefName       string        `json:"baseRefName"`
	HeadRepository    Repository    `json:"headRepository"`
	HeadRepoOwner     RepoOwner     `json:"headRepositoryOwner"`
	IsCrossRepository bool          `json:"isCrossRepository"`
	MergeStateStatus  string        `json:"mergeStateStatus"`
	ReviewDecision    string        `json:"reviewDecision"`
	StatusCheckRollup []CheckRollup `json:"statusCheckRollup"`
}

type Repository struct {
	Name  string    `json:"name"`
	Owner RepoOwner `json:"owner"`
}

type RepoOwner struct {
	Login string `json:"login"`
}

type CheckRollup struct {
	Name       string `json:"name"`
	Workflow   string `json:"workflowName"`
	State      string `json:"state"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

type CreatePROptions struct {
	RepoArg  string
	Title    string
	BodyFile string
	Base     string
	Head     string
}

func New() Client {
	return Client{Runner: execx.Runner{}}
}

func EnsureExists() error {
	_, err := execx.LookPath("gh")
	return err
}

func (c Client) run(ctx context.Context, dir string, args ...string) (execx.Result, error) {
	return c.Runner.Run(ctx, execx.Command{Name: "gh", Args: args, Dir: dir})
}

func (c Client) AuthStatus(ctx context.Context, host string) error {
	args := []string{"auth", "status"}
	if host != "" {
		args = append(args, "--hostname", host)
	}
	_, err := c.run(ctx, "", args...)
	return err
}

func (c Client) IssueView(ctx context.Context, dir, repoArg string, issue int) (Issue, error) {
	args := []string{"issue", "view", strconv.Itoa(issue), "--json", "body,title,url"}
	args = withRepo(args, repoArg)
	result, err := c.run(ctx, dir, args...)
	if err != nil {
		return Issue{}, err
	}
	var out Issue
	if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
		return Issue{}, fmt.Errorf("parse gh issue JSON: %w", err)
	}
	return out, nil
}

func (c Client) IssueComment(ctx context.Context, dir, repoArg string, issue int, body string) error {
	args := []string{"issue", "comment", strconv.Itoa(issue), "--body", body}
	args = withRepo(args, repoArg)
	_, err := c.run(ctx, dir, args...)
	return err
}

func (c Client) CreatePR(ctx context.Context, dir string, opts CreatePROptions) (string, int, error) {
	args := []string{"pr", "create", "--title", opts.Title, "--body-file", opts.BodyFile, "--base", opts.Base, "--head", opts.Head}
	args = withRepo(args, opts.RepoArg)
	result, err := c.run(ctx, dir, args...)
	if err != nil {
		return "", 0, err
	}
	url := strings.TrimSpace(result.Stdout)
	return url, PRNumberFromURL(url), nil
}

func (c Client) CreatePRWithBody(ctx context.Context, dir string, opts CreatePROptions, body string) (string, int, error) {
	tmp, err := os.CreateTemp("", "yard-pr-body-*.md")
	if err != nil {
		return "", 0, fmt.Errorf("create PR body temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(body); err != nil {
		_ = tmp.Close()
		return "", 0, fmt.Errorf("write PR body temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", 0, fmt.Errorf("close PR body temp file: %w", err)
	}
	opts.BodyFile = tmp.Name()
	return c.CreatePR(ctx, dir, opts)
}

func (c Client) PRView(ctx context.Context, dir, repoArg string, pr int) (PullRequest, error) {
	args := []string{"pr", "view", strconv.Itoa(pr), "--json", prJSONFields}
	args = withRepo(args, repoArg)
	result, err := c.run(ctx, dir, args...)
	if err != nil {
		return PullRequest{}, err
	}
	return ParsePRView(result.Stdout)
}

const prJSONFields = "number,title,url,state,headRefName,headRefOid,baseRefName,headRepository,headRepositoryOwner,isCrossRepository,mergeStateStatus,reviewDecision,statusCheckRollup"

func (c Client) PRForBranch(ctx context.Context, dir, repoArg, branch, base string) (PullRequest, bool, error) {
	if branch == "" {
		return PullRequest{}, false, nil
	}
	args := []string{"pr", "list", "--head", branch, "--state", "open", "--limit", "100", "--json", prJSONFields}
	args = withRepo(args, repoArg)
	result, err := c.run(ctx, dir, args...)
	if err != nil {
		return PullRequest{}, false, err
	}
	var prs []PullRequest
	if err := json.Unmarshal([]byte(result.Stdout), &prs); err != nil {
		return PullRequest{}, false, fmt.Errorf("parse gh PR list JSON: %w", err)
	}
	if len(prs) == 0 {
		return PullRequest{}, false, nil
	}
	expectedOwner, expectedRepo := repoArgOwnerName(repoArg)
	for _, pr := range prs {
		if prMatchesBranch(pr, branch, base, expectedOwner, expectedRepo) {
			return pr, true, nil
		}
	}
	return PullRequest{}, false, fmt.Errorf("open PR(s) for branch %q did not match base %q and repository %q", branch, base, repoArg)
}

func (c Client) PRChecks(ctx context.Context, dir, repoArg string, pr int) (string, error) {
	args := []string{"pr", "checks", strconv.Itoa(pr)}
	args = withRepo(args, repoArg)
	result, err := c.run(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

func (c Client) PRCheckout(ctx context.Context, dir, repoArg string, pr int, detach bool) error {
	args := []string{"pr", "checkout", strconv.Itoa(pr)}
	if detach {
		args = append(args, "--detach")
	}
	args = withRepo(args, repoArg)
	_, err := c.run(ctx, dir, args...)
	return err
}

func ParsePRView(output string) (PullRequest, error) {
	var pr PullRequest
	if err := json.Unmarshal([]byte(output), &pr); err != nil {
		return PullRequest{}, fmt.Errorf("parse gh PR JSON: %w", err)
	}
	return pr, nil
}

func PRNumberFromURL(url string) int {
	re := regexp.MustCompile(`/pull/(\d+)(?:$|[/?#])`)
	matches := re.FindStringSubmatch(url)
	if len(matches) != 2 {
		return 0
	}
	number, _ := strconv.Atoi(matches[1])
	return number
}

func withRepo(args []string, repoArg string) []string {
	if repoArg == "" {
		return args
	}
	return append(args, "--repo", repoArg)
}

func prMatchesBranch(pr PullRequest, branch, base, owner, repo string) bool {
	if pr.HeadRefName != branch {
		return false
	}
	if base != "" && pr.BaseRefName != base {
		return false
	}
	if owner == "" && repo == "" {
		return true
	}
	if pr.IsCrossRepository {
		return false
	}
	prOwner := pr.HeadRepoOwner.Login
	if prOwner == "" {
		prOwner = pr.HeadRepository.Owner.Login
	}
	if prOwner == "" || !strings.EqualFold(prOwner, owner) {
		return false
	}
	return pr.HeadRepository.Name != "" && strings.EqualFold(pr.HeadRepository.Name, repo)
}

func repoArgOwnerName(repoArg string) (string, string) {
	parts := strings.Split(strings.Trim(repoArg, "/"), "/")
	if len(parts) < 2 {
		return "", ""
	}
	return parts[len(parts)-2], parts[len(parts)-1]
}
