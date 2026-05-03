package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultFile = "yard.yaml"

type Config struct {
	Repo          string         `yaml:"repo"`
	BaseBranch    string         `yaml:"base_branch"`
	DefaultRemote string         `yaml:"default_remote"`
	Session       string         `yaml:"session"`
	GitHub        GitHubConfig   `yaml:"github"`
	Worktrees     WorktreeConfig `yaml:"worktrees"`
	Agents        AgentsConfig   `yaml:"agents"`
	Signoff       bool           `yaml:"signoff,omitempty"`
}

type GitHubConfig struct {
	Host  string `yaml:"host,omitempty"`
	Owner string `yaml:"owner"`
	Repo  string `yaml:"repo"`
	Issue int    `yaml:"issue"`
}

type WorktreeConfig struct {
	Root   string `yaml:"root"`
	Prefix string `yaml:"prefix"`
}

type AgentsConfig struct {
	Commander      AgentCommand `yaml:"commander"`
	Implementation AgentCommand `yaml:"implementation"`
	LocalReview    AgentCommand `yaml:"local_review"`
	PRReview       AgentCommand `yaml:"pr_review"`
}

type AgentCommand struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

func Default() Config {
	return Config{
		Repo:          ".",
		BaseBranch:    "main",
		DefaultRemote: "origin",
		Session:       "yard",
		Worktrees: WorktreeConfig{
			Root:   "..",
			Prefix: "yard.",
		},
		Agents: AgentsConfig{
			Commander: AgentCommand{
				Command: "codex",
				Args:    []string{"exec", "--sandbox", "danger-full-access"},
			},
			Implementation: AgentCommand{
				Command: "codex",
				Args:    []string{"exec", "--sandbox", "danger-full-access"},
			},
			LocalReview: AgentCommand{
				Command: "codex",
				Args:    []string{"exec", "--sandbox", "danger-full-access"},
			},
			PRReview: AgentCommand{
				Command: "codex",
				Args:    []string{"exec", "--sandbox", "danger-full-access"},
			},
		},
	}
}

func ApplyDefaults(cfg *Config) {
	defaults := Default()
	if cfg.Repo == "" {
		cfg.Repo = defaults.Repo
	}
	if cfg.BaseBranch == "" {
		cfg.BaseBranch = defaults.BaseBranch
	}
	if cfg.DefaultRemote == "" {
		cfg.DefaultRemote = defaults.DefaultRemote
	}
	if cfg.Session == "" {
		cfg.Session = defaults.Session
	}
	if cfg.Worktrees.Root == "" {
		cfg.Worktrees.Root = defaults.Worktrees.Root
	}
	if cfg.Worktrees.Prefix == "" {
		cfg.Worktrees.Prefix = defaults.Worktrees.Prefix
	}
	if cfg.Agents.Commander.Command == "" {
		cfg.Agents.Commander = defaults.Agents.Commander
	}
	if cfg.Agents.Implementation.Command == "" {
		cfg.Agents.Implementation = defaults.Agents.Implementation
	}
	if cfg.Agents.LocalReview.Command == "" {
		cfg.Agents.LocalReview = defaults.Agents.LocalReview
	}
	if cfg.Agents.PRReview.Command == "" {
		cfg.Agents.PRReview = defaults.Agents.PRReview
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	ApplyDefaults(&cfg)
	return cfg, nil
}

func LoadOrDefault(path string) (Config, error) {
	cfg, err := Load(path)
	if err == nil {
		return cfg, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		cfg := Default()
		ApplyDefaults(&cfg)
		return cfg, nil
	}
	return Config{}, err
}

func Save(path string, cfg Config) error {
	ApplyDefaults(&cfg)
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func BaseDir(configPath string) string {
	dir := filepath.Dir(configPath)
	if dir == "" {
		return "."
	}
	return dir
}

func ResolvePath(configPath, value string) string {
	if value == "" {
		return ""
	}
	expanded := os.ExpandEnv(value)
	if strings.HasPrefix(expanded, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~/"))
		}
	}
	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded)
	}
	return filepath.Clean(filepath.Join(BaseDir(configPath), expanded))
}

func RepoPath(configPath string, cfg Config) string {
	return ResolvePath(configPath, cfg.Repo)
}

func WorktreeRootPath(configPath string, cfg Config) string {
	return ResolvePath(configPath, cfg.Worktrees.Root)
}

func GitHubHost(cfg Config) string {
	host := strings.TrimSpace(cfg.GitHub.Host)
	if host == "" {
		return "github.com"
	}
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimRight(host, "/")
	if host == "" {
		return "github.com"
	}
	return host
}

func GitHubRepoArg(cfg Config) string {
	if cfg.GitHub.Owner == "" || cfg.GitHub.Repo == "" {
		return ""
	}
	if host := GitHubHost(cfg); host != "github.com" {
		return host + "/" + cfg.GitHub.Owner + "/" + cfg.GitHub.Repo
	}
	return cfg.GitHub.Owner + "/" + cfg.GitHub.Repo
}
