package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the per-project workspace binding.
type Config struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	ProjectRoot   string `json:"project_root"`
}

// projectsDir returns the path to the projects directory within arcHome.
func projectsDir(arcHome string) string {
	return filepath.Join(arcHome, "projects")
}

// configPath returns the full path to a project's config.json.
func configPath(arcHome, absProjectPath string) string {
	dirName := PathToProjectDir(absProjectPath)
	return filepath.Join(projectsDir(arcHome), dirName, "config.json")
}

// WriteConfig writes a project config to ~/.arc/projects/<path>/config.json.
func WriteConfig(arcHome, absProjectPath string, cfg *Config) error {
	path := configPath(arcHome, absProjectPath)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// LoadConfig reads a project config from ~/.arc/projects/<path>/config.json.
func LoadConfig(arcHome, absProjectPath string) (*Config, error) {
	path := configPath(arcHome, absProjectPath)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}

	return &cfg, nil
}

// PathToProjectDir converts an absolute filesystem path to a project directory name.
// Replaces "/" with "-", matching the Claude Code ~/.claude/projects/ convention.
// Example: "/home/user/my-repo" → "-home-user-my-repo"
func PathToProjectDir(absPath string) string {
	cleaned := filepath.Clean(absPath)
	return strings.ReplaceAll(cleaned, string(filepath.Separator), "-")
}
