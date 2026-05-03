package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrDefault(t *testing.T) {
	cfg, err := LoadOrDefault(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("LoadOrDefault returned error: %v", err)
	}
	if cfg.BaseBranch != "main" {
		t.Fatalf("BaseBranch = %q; want main", cfg.BaseBranch)
	}
	if cfg.Agents.Implementation.Command != "codex" {
		t.Fatalf("Implementation command = %q; want codex", cfg.Agents.Implementation.Command)
	}
	if cfg.Agents.Commander.Command != "codex" {
		t.Fatalf("Commander command = %q; want codex", cfg.Agents.Commander.Command)
	}
	if len(cfg.Agents.PRReview.Args) != 3 {
		t.Fatalf("PRReview args = %#v; want full-access codex exec defaults", cfg.Agents.PRReview.Args)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "yard.yaml")
	if err := os.WriteFile(path, []byte("repo: ../repo\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Repo != "../repo" {
		t.Fatalf("Repo = %q; want ../repo", cfg.Repo)
	}
	if cfg.DefaultRemote != "origin" {
		t.Fatalf("DefaultRemote = %q; want origin", cfg.DefaultRemote)
	}
	if len(cfg.Agents.Implementation.Args) != 3 {
		t.Fatalf("Implementation args = %#v", cfg.Agents.Implementation.Args)
	}
}

func TestResolvePath(t *testing.T) {
	dir := t.TempDir()
	got := ResolvePath(filepath.Join(dir, "yard.yaml"), "../work")
	want := filepath.Clean(filepath.Join(dir, "../work"))
	if got != want {
		t.Fatalf("ResolvePath = %q; want %q", got, want)
	}
}

func TestGitHubRepoArgIncludesConfiguredEnterpriseHost(t *testing.T) {
	cfg := Config{GitHub: GitHubConfig{Host: "https://ghe.example.com/", Owner: "owner", Repo: "repo"}}
	if got := GitHubRepoArg(cfg); got != "ghe.example.com/owner/repo" {
		t.Fatalf("GitHubRepoArg = %q; want enterprise host repo arg", got)
	}
}

func TestGitHubHostDefaultsAndNormalizes(t *testing.T) {
	if got := GitHubHost(Config{}); got != "github.com" {
		t.Fatalf("GitHubHost default = %q; want github.com", got)
	}
	cfg := Config{GitHub: GitHubConfig{Host: "http://ghe.example.com///"}}
	if got := GitHubHost(cfg); got != "ghe.example.com" {
		t.Fatalf("GitHubHost = %q; want normalized host", got)
	}
	cfg = Config{GitHub: GitHubConfig{Host: "https://github.com/", Owner: "owner", Repo: "repo"}}
	if got := GitHubRepoArg(cfg); got != "owner/repo" {
		t.Fatalf("GitHubRepoArg github.com = %q; want owner/repo", got)
	}
}
