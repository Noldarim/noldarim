// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// AppConfig holds all application configuration.
// It is instantiated by NewConfig() and passed to components that need it (dependency injection).
type AppConfig struct {
	Database    DatabaseConfig    `mapstructure:"database"`
	Log         LogConfig         `mapstructure:"log"`
	Temporal    TemporalConfig    `mapstructure:"temporal"`
	Container   ContainerConfig   `mapstructure:"container"`
	Git         GitConfig         `mapstructure:"git"`
	Server      ServerConfig      `mapstructure:"server"`
	Claude      ClaudeConfig      `mapstructure:"claude"`
	Agent       AgentConfig       `mapstructure:"agent"`
	Hooks       HooksConfig       `mapstructure:"hooks"`
	Pipeline    PipelineConfig    `mapstructure:"pipeline"`
}

// DatabaseConfig holds all database configuration.
type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

// LogConfig holds comprehensive logging configuration
type LogConfig struct {
	Level    string            `mapstructure:"level"`
	Format   string            `mapstructure:"format"`
	Dir      string            `mapstructure:"dir"` // Deprecated, kept for backward compatibility
	Output   []LogOutputConfig `mapstructure:"output"`
	Levels   map[string]string `mapstructure:"levels"`
	Context  LogContextConfig  `mapstructure:"context"`
	Sampling LogSamplingConfig `mapstructure:"sampling"`
}

// LogOutputConfig defines where logs are written
type LogOutputConfig struct {
	Type    string          `mapstructure:"type"` // "file", "console", "syslog"
	Enabled bool            `mapstructure:"enabled"`
	Path    string          `mapstructure:"path"`   // For file output
	Rotate  LogRotateConfig `mapstructure:"rotate"` // For file output
}

// LogRotateConfig defines log rotation settings
type LogRotateConfig struct {
	MaxSizeMB  int  `mapstructure:"max_size_mb"`
	MaxBackups int  `mapstructure:"max_backups"`
	MaxAgeDays int  `mapstructure:"max_age_days"`
	Compress   bool `mapstructure:"compress"`
}

// LogContextConfig defines what context to include in logs
type LogContextConfig struct {
	IncludeCaller     bool   `mapstructure:"include_caller"`
	IncludeTimestamp  bool   `mapstructure:"include_timestamp"`
	IncludeLevel      bool   `mapstructure:"include_level"`
	IncludeStackTrace string `mapstructure:"include_stack_trace"` // Level at which to include stack trace
}

// LogSamplingConfig defines log sampling settings
type LogSamplingConfig struct {
	Enabled    bool          `mapstructure:"enabled"`
	Initial    uint32        `mapstructure:"initial"`
	Thereafter uint32        `mapstructure:"thereafter"`
	Tick       time.Duration `mapstructure:"tick"`
}

// TemporalConfig holds Temporal-related configuration.
type TemporalConfig struct {
	HostPort  string          `mapstructure:"host_port"`
	Namespace string          `mapstructure:"namespace"`
	TaskQueue string          `mapstructure:"task_queue"`
	Worker    WorkerConfig    `mapstructure:"worker"`
	Activity  ActivityOptions `mapstructure:"activity"`
	Workflow  WorkflowOptions `mapstructure:"workflow"`
}

// WorkerConfig holds Temporal worker configuration.
type WorkerConfig struct {
	MaxConcurrentActivityExecutions int     `mapstructure:"max_concurrent_activities"`
	MaxConcurrentWorkflows          int     `mapstructure:"max_concurrent_workflows"`
	ActivitiesPerSecond             float64 `mapstructure:"activities_per_second"`
}

// ActivityOptions holds common activity options.
type ActivityOptions struct {
	StartToCloseTimeout    time.Duration `mapstructure:"start_to_close_timeout"`
	ScheduleToCloseTimeout time.Duration `mapstructure:"schedule_to_close_timeout"`
	HeartbeatTimeout       time.Duration `mapstructure:"heartbeat_timeout"`
	RetryPolicy            RetryPolicy   `mapstructure:"retry_policy"`
}

// RetryPolicy defines retry behavior for activities.
type RetryPolicy struct {
	InitialInterval    time.Duration `mapstructure:"initial_interval"`
	BackoffCoefficient float64       `mapstructure:"backoff_coefficient"`
	MaximumInterval    time.Duration `mapstructure:"maximum_interval"`
	MaximumAttempts    int32         `mapstructure:"maximum_attempts"`
}

// WorkflowOptions holds common workflow options.
type WorkflowOptions struct {
	WorkflowExecutionTimeout time.Duration `mapstructure:"workflow_execution_timeout"`
	WorkflowRunTimeout       time.Duration `mapstructure:"workflow_run_timeout"`
	WorkflowTaskTimeout      time.Duration `mapstructure:"workflow_task_timeout"`
}

// ContainerConfig holds container-related configuration.
type ContainerConfig struct {
	DefaultImage   string            `mapstructure:"default_image"`
	WorkspaceDir   string            `mapstructure:"workspace_dir"`
	DockerHost     string            `mapstructure:"docker_host"`
	NetworkMode    string            `mapstructure:"network_mode"`
	Volumes        []VolumeConfig    `mapstructure:"volumes"`
	Environment    map[string]string `mapstructure:"environment"`
	ResourceLimits ResourceLimits    `mapstructure:"resource_limits"`
	Timeouts       ContainerTimeouts `mapstructure:"timeouts"`
}

// VolumeConfig defines volume mount configuration.
type VolumeConfig struct {
	Host      string `mapstructure:"host"`
	Container string `mapstructure:"container"`
	ReadOnly  bool   `mapstructure:"read_only"`
}

// ResourceLimits defines container resource limits.
type ResourceLimits struct {
	CPUShares  int64 `mapstructure:"cpu_shares"`
	MemoryMB   int64 `mapstructure:"memory_mb"`
	DiskSizeMB int64 `mapstructure:"disk_size_mb"`
}

// ContainerTimeouts defines container operation timeouts.
type ContainerTimeouts struct {
	StopTimeout         time.Duration `mapstructure:"stop_timeout"`
	TaskDuplicateWindow time.Duration `mapstructure:"task_duplicate_window"`
}

// GitConfig holds git-related configuration.
type GitConfig struct {
	WorktreeBasePath                  string `mapstructure:"worktree_base_path"`
	DefaultBranch                     string `mapstructure:"default_branch"`
	CreateGitRepoForProjectIfNotExist bool   `mapstructure:"create_git_repo_for_project_if_not_exist"`
}

// ServerConfig holds server configuration.
type ServerConfig struct {
	Host           string   `mapstructure:"host"`
	Port           int      `mapstructure:"port"`
	AllowedOrigins []string `mapstructure:"allowed_origins"` // Empty = allow all (development); set for production
}

// ClaudeConfig holds Claude-related configuration.
type ClaudeConfig struct {
	ClaudeJSONHostPath string `mapstructure:"claude_json_host_path"`
}

// AgentConfig holds default AI agent configuration for task processing.
// This defines the default behavior when a task is created without explicit agent configuration.
type AgentConfig struct {
	DefaultTool    string                 `mapstructure:"default_tool"`    // Tool name: "claude", "gemini", etc.
	DefaultVersion string                 `mapstructure:"default_version"` // Tool version: "4.5"
	PromptTemplate string                 `mapstructure:"prompt_template"` // Template with {{.variable}} placeholders
	Variables      map[string]string      `mapstructure:"variables"`       // Default values for template variables
	ToolOptions    map[string]interface{} `mapstructure:"tool_options"`    // CLI flags and options (e.g., model, custom flags)
	FlagFormat     string                 `mapstructure:"flag_format"`     // Format for CLI flags: "space" (--flag value) or "equals" (--flag=value)
}

// HooksConfig holds configuration for Claude Code hooks.
type HooksConfig struct {
	EnableLogging bool   `mapstructure:"enable_logging"` // Enable debug logging in hook script
	ScriptPath    string `mapstructure:"script_path"`    // Path for hook script in container (default: ~/.noldarim/bin/noldarim-hook.sh)
}

// PipelineConfig holds default configuration for pipeline execution.
type PipelineConfig struct {
	PromptPrefix string `mapstructure:"prompt_prefix"` // Default prefix prepended to all step prompts
	PromptSuffix string `mapstructure:"prompt_suffix"` // Default suffix appended to all step prompts (e.g., summary instruction)
}

// NewConfig creates a new AppConfig by reading from a file, environment variables,
// and applying defaults. This function replaces the global Init().
func NewConfig(configPath string) (*AppConfig, error) {
	// Create a new config struct with default values
	cfg := defaultConfig()

	v := viper.New()

	// Set config file if provided, otherwise search in standard locations
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/noldarim/")
		v.AddConfigPath("$HOME/.noldarim")
	}

	// Configure viper to use environment variables
	v.SetEnvPrefix("NOLDARIM")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read the config file. It's okay if it doesn't exist.
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal the viper configuration into our config struct.
	// This will overwrite the default values with any values found in the config file or env vars.
	// We use a decoder hook to correctly handle nested structs.
	if err := v.Unmarshal(&cfg, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	))); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand paths that may contain ~ or environment variables
	cfg.expandPaths()

	// Validate the final configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// defaultConfig returns an AppConfig with default values.
// This is more type-safe than using viper.SetDefault().
func defaultConfig() AppConfig {
	return AppConfig{
		Database: DatabaseConfig{
			Driver:   "sqlite",
			Database: "noldarim.db",
			Host:     "localhost",
			Port:     5432,
			SSLMode:  "disable",
		},
		Log: LogConfig{
			Level:  "INFO",
			Format: "console",
			Dir:    "./logs", // Backward compatibility
			Output: []LogOutputConfig{
				{
					Type:    "file",
					Enabled: true,
					Path:    "./logs/noldarim.log",
					Rotate: LogRotateConfig{
						MaxSizeMB:  100,
						MaxBackups: 7,
						MaxAgeDays: 30,
						Compress:   true,
					},
				},
				{
					Type:    "console",
					Enabled: false, // Disabled by default for TUI
				},
			},
			Levels: map[string]string{
				"orchestrator": "INFO",
				"temporal":     "WARN",
				"tui":          "WARN",
				"database":     "INFO",
				"git":          "INFO",
				"container":    "INFO",
				"api":          "INFO",
			},
			Context: LogContextConfig{
				IncludeCaller:     true,
				IncludeTimestamp:  true,
				IncludeLevel:      true,
				IncludeStackTrace: "ERROR",
			},
			Sampling: LogSamplingConfig{
				Enabled:    false,
				Initial:    100,
				Thereafter: 100,
				Tick:       time.Second,
			},
		},
		Temporal: TemporalConfig{
			HostPort:  "localhost:7233",
			Namespace: "default",
			TaskQueue: "noldarim-task-queue",
			Worker: WorkerConfig{
				MaxConcurrentActivityExecutions: 100,
				MaxConcurrentWorkflows:          100,
				ActivitiesPerSecond:             100000,
			},
			Activity: ActivityOptions{
				StartToCloseTimeout:    30 * time.Second,
				ScheduleToCloseTimeout: 5 * time.Minute,
				HeartbeatTimeout:       5 * time.Second,
				RetryPolicy: RetryPolicy{
					InitialInterval:    time.Second,
					BackoffCoefficient: 2.0,
					MaximumInterval:    time.Minute,
					MaximumAttempts:    3,
				},
			},
			Workflow: WorkflowOptions{
				WorkflowExecutionTimeout: 24 * time.Hour,
				WorkflowRunTimeout:       24 * time.Hour,
				WorkflowTaskTimeout:      10 * time.Second,
			},
		},
		Container: ContainerConfig{
			DefaultImage: "ubuntu:22.04",
			WorkspaceDir: "/workspace",
			DockerHost:   "unix:///var/run/docker.sock",
			ResourceLimits: ResourceLimits{
				CPUShares:  1024,
				MemoryMB:   2048,
				DiskSizeMB: 10240,
			},
			Timeouts: ContainerTimeouts{
				StopTimeout:         10 * time.Second,
				TaskDuplicateWindow: 5 * time.Minute,
			},
		},
		Git: GitConfig{
			WorktreeBasePath:                  "./worktrees",
			DefaultBranch:                     "main",
			CreateGitRepoForProjectIfNotExist: true,
		},
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Claude: ClaudeConfig{
			ClaudeJSONHostPath: "$HOME/.claude.json",
		},
		Agent: AgentConfig{
			DefaultTool:    "claude",
			DefaultVersion: "4.5",
			PromptTemplate: `Please complete the task described below.

Task: {{.title}}
Description: {{.description}}

The task details have been written to: {{.task_file}}

Please read the task file for complete information and execute the requested work in the workspace directory.`,
			Variables: map[string]string{
				"title":       "",
				"description": "",
				"task_file":   "",
			},
			ToolOptions: map[string]interface{}{
				"model": "claude-sonnet-4-5",
			},
			FlagFormat: "space", // Default: --flag value
		},
		Hooks: HooksConfig{
			EnableLogging: false,
			ScriptPath:    "", // Empty means use default: /home/noldarim/.noldarim/bin/noldarim-hook.sh
		},
		Pipeline: PipelineConfig{
			PromptPrefix: "",
			PromptSuffix: `

When you complete this task, end your response with a summary in this exact format:
---SUMMARY---
{"reason": "brief explanation of why these changes were needed", "changes": ["change 1", "change 2", "change 3"]}
---END SUMMARY---
`,
		},
	}
}

// expandPaths expands ~ and environment variables in path configuration values
func (c *AppConfig) expandPaths() {
	// Expand Claude config path
	if c.Claude.ClaudeJSONHostPath != "" {
		c.Claude.ClaudeJSONHostPath = expandPath(c.Claude.ClaudeJSONHostPath)
	}

	// Expand Git worktree base path
	if c.Git.WorktreeBasePath != "" {
		c.Git.WorktreeBasePath = expandPath(c.Git.WorktreeBasePath)
	}

	// Expand Docker host path
	if c.Container.DockerHost != "" {
		c.Container.DockerHost = expandPath(c.Container.DockerHost)
	}
}

// expandPath expands ~ to home directory and environment variables
func expandPath(path string) string {
	if path == "" {
		return path
	}

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(homeDir, path[1:])
		}
	}

	// Expand environment variables
	path = os.ExpandEnv(path)

	return path
}

// validate checks if the configuration is valid.
func (c *AppConfig) validate() error {
	if c.Database.Driver == "" {
		return errors.New("database driver is required")
	}

	validLogLevels := map[string]bool{
		"DEBUG": true, "INFO": true, "WARN": true, "ERROR": true, "FATAL": true, "PANIC": true,
	}
	if !validLogLevels[strings.ToUpper(c.Log.Level)] {
		return fmt.Errorf("invalid log level: %s", c.Log.Level)
	}

	if c.Container.DefaultImage == "" {
		return errors.New("container default_image is required")
	}

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Validate agent configuration
	if c.Agent.DefaultTool == "" {
		return errors.New("agent.default_tool is required")
	}
	if c.Agent.PromptTemplate == "" {
		return errors.New("agent.prompt_template is required")
	}
	if c.Agent.FlagFormat != "" && c.Agent.FlagFormat != "space" && c.Agent.FlagFormat != "equals" {
		return fmt.Errorf("agent.flag_format must be 'space' or 'equals', got: %s", c.Agent.FlagFormat)
	}

	return nil
}

// GetDSN returns the database connection string.
func (dc *DatabaseConfig) GetDSN() string {
	switch dc.Driver {
	case "sqlite":
		dsn := dc.Database
		if dsn == ":memory:" {
			dsn = "file::memory:?cache=shared"
		}
		return dsn
	case "postgres":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			dc.Host, dc.Port, dc.Username, dc.Password, dc.Database, dc.SSLMode)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			dc.Username, dc.Password, dc.Host, dc.Port, dc.Database)
	default:
		// Fallback for other drivers that might just use a connection string directly
		return dc.Database
	}
}
