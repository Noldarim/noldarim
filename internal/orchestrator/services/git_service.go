// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/logger"
)

var (
	log     *zerolog.Logger
	logOnce sync.Once
)

func getLog() *zerolog.Logger {
	logOnce.Do(func() {
		l := logger.GetGitLogger().With().Str("component", "service").Logger()
		log = &l
	})
	return log
}

// GitService handles git operations for projects and tasks
type GitService struct {
	workDir string
	config  *config.AppConfig
}

// Security constants for validation
const (
	maxPathLength          = 4096
	maxBranchNameLength    = 250
	maxCommitMessageLength = 8192
	maxAgentIDLength       = 100
)

// Regular expressions for validation
var (
	// Safe branch name pattern: alphanumeric, hyphens, underscores, forward slashes
	branchNameRegex = regexp.MustCompile(`^[a-zA-Z0-9/_-]+$`)

	// Safe agent ID pattern: alphanumeric, hyphens, underscores
	agentIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	// Dangerous patterns to reject in commit messages
	dangerousPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\$\(`), // Command substitution
		regexp.MustCompile(`;`),    // Command chaining
		regexp.MustCompile(`\|\|`), // Logical OR
		regexp.MustCompile(`&&`),   // Logical AND
		regexp.MustCompile(`\|`),   // Pipe
		regexp.MustCompile(`>`),    // Redirect
		regexp.MustCompile(`<`),    // Redirect
	}
)

// Allowed git operations for security
var allowedGitOperations = map[string]bool{
	"init":      true,
	"add":       true,
	"commit":    true,
	"checkout":  true,
	"branch":    true,
	"status":    true,
	"rev-parse": true,
	"diff":      true,
	"log":       true,
	"show-ref":  true,
	"worktree":  true,
	"stash":     true,
	"reset":     true,
	"clean":     true,
	"remote":    true,
	"config":    true,
}

// NewGitService creates a new git service with the provided repository path
// If createIfNotExist is true and the path is not a git repository, it will be initialized
func NewGitService(repoPath string, createIfNotExist bool) (*GitService, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path cannot be empty")
	}

	// Validate and get absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create the GitService instance to use its methods
	gs := &GitService{
		workDir: absPath,
	}

	// Check if directory exists, create if needed
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		if createIfNotExist {
			if err := os.MkdirAll(absPath, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory: %w", err)
			}
		} else {
			return nil, fmt.Errorf("repository path does not exist: %s", absPath)
		}
	}

	// Initialize repository if createIfNotExist is true
	// InitRepository handles all cases: no repo, empty repo, repo with commits
	if createIfNotExist {
		ctx := context.Background()
		if err := gs.InitRepository(ctx, absPath); err != nil {
			return nil, fmt.Errorf("failed to initialize git repository: %w", err)
		}
	} else {
		// Only validate that it's a git repository if we're not creating it
		if !gs.isGitRepository(absPath) {
			return nil, fmt.Errorf("not a git repository: %s", absPath)
		}
	}

	return gs, nil
}

// NewGitServiceWithConfig creates a new git service with configuration and repository path
// If createIfNotExist is true and the path is not a git repository, it will be initialized
func NewGitServiceWithConfig(repoPath string, cfg *config.AppConfig, createIfNotExist bool) (*GitService, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path cannot be empty")
	}

	// Validate and get absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create the GitService instance to use its methods
	gs := &GitService{
		workDir: absPath,
		config:  cfg,
	}

	// Check if directory exists, create if needed
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		if createIfNotExist {
			if err := os.MkdirAll(absPath, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory: %w", err)
			}
		} else {
			return nil, fmt.Errorf("repository path does not exist: %s", absPath)
		}
	}

	// Initialize repository if createIfNotExist is true
	// InitRepository handles all cases: no repo, empty repo, repo with commits
	if createIfNotExist {
		ctx := context.Background()
		if err := gs.InitRepository(ctx, absPath); err != nil {
			return nil, fmt.Errorf("failed to initialize git repository: %w", err)
		}
	} else {
		// Only validate that it's a git repository if we're not creating it
		if !gs.isGitRepository(absPath) {
			return nil, fmt.Errorf("not a git repository: %s", absPath)
		}
	}

	return gs, nil
}

// Security validation functions

// validateRepoPath validates and canonicalizes repository paths
func (gs *GitService) validateRepoPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("repository path cannot be empty")
	}

	if len(path) > maxPathLength {
		return "", fmt.Errorf("repository path too long: %d characters (max: %d)", len(path), maxPathLength)
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check for directory traversal before cleaning
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path contains invalid directory traversal")
	}

	// Clean the path to remove any .. or . elements
	cleanPath := filepath.Clean(absPath)

	return cleanPath, nil
}

// validateBranchName validates branch names for security
func validateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	if len(name) > maxBranchNameLength {
		return fmt.Errorf("branch name too long: %d characters (max: %d)", len(name), maxBranchNameLength)
	}

	// Reject names that start with special characters (check first for better error messages)
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, ".") {
		return fmt.Errorf("branch name cannot start with '-' or '.'")
	}

	// Check for dangerous characters
	if !branchNameRegex.MatchString(name) {
		return fmt.Errorf("branch name contains invalid characters: %s", name)
	}

	return nil
}

// validateCommitMessage validates commit messages for security
func validateCommitMessage(message string) error {
	if message == "" {
		return fmt.Errorf("commit message cannot be empty")
	}

	if len(message) > maxCommitMessageLength {
		return fmt.Errorf("commit message too long: %d characters (max: %d)", len(message), maxCommitMessageLength)
	}

	// Check for dangerous patterns
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(message) {
			return fmt.Errorf("commit message contains dangerous pattern: %s", pattern.String())
		}
	}

	return nil
}

// validateConfigKey validates git configuration keys
func (gs *GitService) validateConfigKey(key string) error {
	if len(key) == 0 {
		return fmt.Errorf("config key cannot be empty")
	}

	if len(key) > 250 {
		return fmt.Errorf("config key too long: %d characters (max: 250)", len(key))
	}

	// Allow common git config keys
	// Pattern: section.subsection.key or section.key
	configKeyRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*(\.[a-zA-Z][a-zA-Z0-9_-]*)*$`)
	if !configKeyRegex.MatchString(key) {
		return fmt.Errorf("invalid config key format: %s", key)
	}

	return nil
}

// validateConfigValue validates git configuration values
func (gs *GitService) validateConfigValue(value string) error {
	if len(value) > 1000 {
		return fmt.Errorf("config value too long: %d characters (max: 1000)", len(value))
	}

	// Check for dangerous patterns in config values
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(value) {
			return fmt.Errorf("config value contains dangerous pattern: %s", pattern.String())
		}
	}

	return nil
}

// validateAgentID validates agent IDs for security
func validateAgentID(agentID string) error {
	if agentID == "" {
		return fmt.Errorf("agent ID cannot be empty")
	}

	if len(agentID) > maxAgentIDLength {
		return fmt.Errorf("agent ID too long: %d characters (max: %d)", len(agentID), maxAgentIDLength)
	}

	if !agentIDRegex.MatchString(agentID) {
		return fmt.Errorf("agent ID contains invalid characters: %s", agentID)
	}

	return nil
}

// validateCommitHash validates commit hashes
func validateCommitHash(hash string) error {
	if hash == "" {
		return fmt.Errorf("commit hash cannot be empty")
	}

	// Git commit hashes are typically 40 characters (SHA-1) or 64 characters (SHA-256)
	if len(hash) != 40 && len(hash) != 64 {
		return fmt.Errorf("invalid commit hash length: %d", len(hash))
	}

	// Check for valid hex characters
	for _, char := range hash {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return fmt.Errorf("commit hash contains invalid characters: %s", hash)
		}
	}

	return nil
}

// getSafeEnvironment returns a minimal, safe environment for git commands
func (gs *GitService) getSafeEnvironment() []string {
	return []string{
		"HOME=" + os.Getenv("HOME"),
		"USER=" + os.Getenv("USER"),
		"PATH=" + os.Getenv("PATH"),
		"LANG=" + os.Getenv("LANG"),
		"LC_ALL=" + os.Getenv("LC_ALL"),
		// Git-specific environment variables
		"GIT_TERMINAL_PROMPT=0", // Disable interactive prompts
		"GIT_ASKPASS=",          // Disable password prompts
	}
}

// buildSafeGitCommand builds a git command with security validations
func (gs *GitService) buildSafeGitCommand(ctx context.Context, workDir string, args ...string) (*exec.Cmd, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no git command specified")
	}

	// Validate the git operation
	operation := args[0]
	if !allowedGitOperations[operation] {
		return nil, fmt.Errorf("git operation not allowed: %s", operation)
	}

	// Validate working directory
	validatedWorkDir, err := gs.validateRepoPath(workDir)
	if err != nil {
		return nil, fmt.Errorf("invalid working directory: %w", err)
	}

	// Log the operation for security monitoring
	getLog().Debug().Str("operation", operation).Strs("args", args).Str("work_dir", validatedWorkDir).Msg("Git operation")

	// Build the command
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = validatedWorkDir
	cmd.Env = gs.getSafeEnvironment()

	return cmd, nil
}

// runSafeGitCommand executes a git command with security validations and timeout
func (gs *GitService) runSafeGitCommand(ctx context.Context, workDir string, args ...string) error {
	// Set maximum execution time to prevent hanging
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd, err := gs.buildSafeGitCommand(ctx, workDir, args...)
	if err != nil {
		return err
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git command failed: %s, output: %s", err, string(output))
	}

	return nil
}

// GitState represents the current state of a git repository
type GitState struct {
	RepoPath      string
	Branch        string
	CommitHash    string
	IsClean       bool
	RemoteURL     string
	HasUntracked  bool
	HasUnstaged   bool
	HasStaged     bool
	WorktreePaths []string
}

// GitOperation represents a git operation that can be executed
type GitOperation struct {
	Type        string
	Description string
	Command     []string
	WorkingDir  string
}

// InitRepository initializes a new git repository
func (gs *GitService) InitRepository(ctx context.Context, repoPath string) error {
	getLog().Debug().Str("repo_path", repoPath).Msg("Initializing git repository")

	// Validate repository path
	validatedPath, err := gs.validateRepoPath(repoPath)
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(validatedPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Check if repository exists and has commits
	isRepo := gs.isGitRepository(validatedPath)
	var hasCommits bool

	if isRepo {
		// Check if repository has commits
		_, err := gs.getCurrentCommit(ctx, validatedPath)
		hasCommits = (err == nil)
	}

	// Handle different scenarios
	if !isRepo {
		// Case 1: No repository exists - initialize and create initial commit
		if err := gs.runSafeGitCommand(ctx, validatedPath, "init"); err != nil {
			return fmt.Errorf("failed to initialize git repository: %w", err)
		}
		getLog().Info().Str("repo_path", validatedPath).Msg("Initialized new git repository")

		if err := gs.CreateInitialNoldarimCommit(ctx, validatedPath); err != nil {
			return fmt.Errorf("failed to create initial noldarim commit: %w", err)
		}
	} else if !hasCommits {
		// Case 2: Repository exists but has no commits - create initial commit
		getLog().Info().Str("repo_path", validatedPath).Msg("Repository exists but has no commits, creating initial noldarim commit")

		if err := gs.CreateInitialNoldarimCommit(ctx, validatedPath); err != nil {
			return fmt.Errorf("failed to create initial noldarim commit: %w", err)
		}
	} else {
		// Case 3: Repository exists and has commits - do nothing
		getLog().Info().Str("repo_path", validatedPath).Msg("Repository already initialized with commits, skipping noldarim initialization")
	}

	return nil
}

// CreateInitialNoldarimCommit creates the initial noldarim.md file and commits it
func (gs *GitService) CreateInitialNoldarimCommit(ctx context.Context, repoPath string) error {
	getLog().Debug().Str("repo_path", repoPath).Msg("Creating initial noldarim commit")

	// Validate repository path
	validatedPath, err := gs.validateRepoPath(repoPath)
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// Create noldarim.md file
	noldarimFilePath := filepath.Join(validatedPath, "noldarim.md")
	noldarimContent := `# noldarim Project

This is a noldarim project repository.
`
	if err := os.WriteFile(noldarimFilePath, []byte(noldarimContent), 0644); err != nil {
		return fmt.Errorf("failed to create noldarim.md file: %w", err)
	}

	// Create initial commit (CreateCommit method handles adding files and committing)
	if err := gs.CreateCommit(ctx, validatedPath, "noldarim project initialized"); err != nil {
		return fmt.Errorf("failed to create initial commit: %w", err)
	}

	getLog().Info().Str("repo_path", validatedPath).Msg("Successfully created initial noldarim commit")
	return nil
}

// SetConfig sets a git configuration option for the repository
func (gs *GitService) SetConfig(ctx context.Context, repoPath, key, value string) error {
	getLog().Debug().Str("repo_path", repoPath).Str("key", key).Str("value", value).Msg("Setting git config")

	// Validate repository path
	validatedPath, err := gs.validateRepoPath(repoPath)
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// Validate config key and value for security
	if err := gs.validateConfigKey(key); err != nil {
		return fmt.Errorf("invalid config key: %w", err)
	}

	if err := gs.validateConfigValue(value); err != nil {
		return fmt.Errorf("invalid config value: %w", err)
	}

	// Set git configuration using safe command
	if err := gs.runSafeGitCommand(ctx, validatedPath, "config", key, value); err != nil {
		return fmt.Errorf("failed to set git config %s: %w", key, err)
	}

	getLog().Debug().Str("repo_path", validatedPath).Str("key", key).Str("value", value).Msg("Successfully set git config")
	return nil
}

// ValidateRepository validates the state of a git repository
func (gs *GitService) ValidateRepository(ctx context.Context, repoPath string) (*GitState, error) {
	getLog().Debug().Str("repo_path", repoPath).Msg("Validating git repository")

	// Validate repository path
	validatedPath, err := gs.validateRepoPath(repoPath)
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}

	if !gs.isGitRepository(validatedPath) {
		return nil, fmt.Errorf("not a git repository: %s", validatedPath)
	}

	state := &GitState{
		RepoPath: validatedPath,
	}

	// Get current branch
	branch, err := gs.getCurrentBranch(ctx, validatedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}
	state.Branch = branch

	// Get current commit hash
	commitHash, err := gs.getCurrentCommit(ctx, validatedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get current commit: %w", err)
	}
	state.CommitHash = commitHash

	// Check if working directory is clean
	isClean, err := gs.IsWorkingDirectoryClean(ctx, validatedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check working directory status: %w", err)
	}
	state.IsClean = isClean

	// Get remote URL
	remoteURL, err := gs.getRemoteURL(ctx, validatedPath)
	if err != nil {
		getLog().Debug().Str("repo_path", validatedPath).Msg("No remote URL found for repository")
		state.RemoteURL = ""
	} else {
		state.RemoteURL = remoteURL
	}

	// Get worktree paths
	worktrees, err := gs.ListWorktrees(ctx)
	if err != nil {
		getLog().Debug().Err(err).Msg("Failed to get worktrees")
		state.WorktreePaths = []string{}
	} else {
		state.WorktreePaths = worktrees
	}

	getLog().Debug().Interface("state", state).Msg("Repository validation complete")
	return state, nil
}

// CreateCommit creates a new commit with the given message
func (gs *GitService) CreateCommit(ctx context.Context, repoPath, message string) error {
	getLog().Debug().Str("repo_path", repoPath).Msg("Creating commit in repository")

	// Validate repository path
	validatedPath, err := gs.validateRepoPath(repoPath)
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// Validate commit message
	if err := validateCommitMessage(message); err != nil {
		return fmt.Errorf("invalid commit message: %w", err)
	}

	// Add all changes
	if err := gs.runSafeGitCommand(ctx, validatedPath, "add", "."); err != nil {
		return fmt.Errorf("failed to add changes: %w", err)
	}

	// Check if there are changes to commit
	hasChanges, err := gs.hasChangesToCommit(ctx, validatedPath)
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}

	if !hasChanges {
		getLog().Debug().Str("repo_path", validatedPath).Msg("No changes to commit in repository")
		return nil
	}

	// Create commit
	if err := gs.runSafeGitCommand(ctx, validatedPath, "commit", "-m", message); err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	getLog().Info().Str("repo_path", validatedPath).Msg("Successfully created commit in repository")
	return nil
}

// CommitSpecificFiles commits only the specified files with the given message
func (gs *GitService) CommitSpecificFiles(ctx context.Context, repoPath string, fileNames []string, message string) error {
	getLog().Debug().Str("repo_path", repoPath).Strs("files", fileNames).Msg("Committing specific files in repository")

	// Validate repository path
	validatedPath, err := gs.validateRepoPath(repoPath)
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// Validate commit message
	if err := validateCommitMessage(message); err != nil {
		return fmt.Errorf("invalid commit message: %w", err)
	}

	// Validate that we have files to commit
	if len(fileNames) == 0 {
		return fmt.Errorf("no files specified to commit")
	}

	// Check if files exist before staging
	for _, fileName := range fileNames {
		fullPath := filepath.Join(validatedPath, fileName)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", fileName)
		}
	}

	// Stage the specified files using git add
	addArgs := append([]string{"add"}, fileNames...)
	if err := gs.runSafeGitCommand(ctx, validatedPath, addArgs...); err != nil {
		return fmt.Errorf("failed to stage files: %w", err)
	}

	// Check if there are changes to commit
	hasChanges, err := gs.hasChangesToCommit(ctx, validatedPath)
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}

	if !hasChanges {
		getLog().Debug().Str("repo_path", validatedPath).Strs("files", fileNames).Msg("No changes to commit for specified files")
		return nil
	}

	// Create commit
	if err := gs.runSafeGitCommand(ctx, validatedPath, "commit", "-m", message); err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	getLog().Info().Str("repo_path", validatedPath).Strs("files", fileNames).Msg("Successfully committed specific files in repository")
	return nil
}

// CreateBranch creates a new branch from the current branch
func (gs *GitService) CreateBranch(ctx context.Context, repoPath, branchName string) error {
	getLog().Debug().Msgf("Creating branch '%s' in repository: %s", branchName, repoPath)

	// Validate repository path
	validatedPath, err := gs.validateRepoPath(repoPath)
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// Validate branch name
	if err := validateBranchName(branchName); err != nil {
		return fmt.Errorf("invalid branch name: %w", err)
	}

	// Check if branch already exists
	exists, err := gs.branchExists(ctx, validatedPath, branchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}

	if exists {
		getLog().Debug().Msgf("Branch '%s' already exists in repository: %s", branchName, validatedPath)
		return nil
	}

	// Create branch
	if err := gs.runSafeGitCommand(ctx, validatedPath, "checkout", "-b", branchName); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	getLog().Info().Msgf("Successfully created branch '%s' in repository: %s", branchName, validatedPath)
	return nil
}

// CreateBranchFromRef creates a new branch from a specific reference
func (gs *GitService) CreateBranchFromRef(ctx context.Context, repoPath, branchName, ref string) error {
	getLog().Debug().Msgf("Creating branch '%s' from ref '%s' in repository: %s", branchName, ref, repoPath)

	// Validate repository path
	validatedPath, err := gs.validateRepoPath(repoPath)
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// Validate branch name
	if err := validateBranchName(branchName); err != nil {
		return fmt.Errorf("invalid branch name: %w", err)
	}

	// Validate ref (could be commit hash or branch name)
	if ref == "" {
		return fmt.Errorf("ref cannot be empty")
	}

	// Check if branch already exists
	exists, err := gs.branchExists(ctx, validatedPath, branchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}

	if exists {
		getLog().Debug().Msgf("Branch '%s' already exists in repository: %s", branchName, validatedPath)
		return nil
	}

	// Create branch from ref
	if err := gs.runSafeGitCommand(ctx, validatedPath, "checkout", "-b", branchName, ref); err != nil {
		return fmt.Errorf("failed to create branch from ref: %w", err)
	}

	getLog().Info().Msgf("Successfully created branch '%s' from ref '%s' in repository: %s", branchName, ref, validatedPath)
	return nil
}

// SwitchBranch switches to the specified branch
func (gs *GitService) SwitchBranch(ctx context.Context, repoPath, branchName string) error {
	getLog().Debug().Msgf("Switching to branch '%s' in repository: %s", branchName, repoPath)

	// Validate repository path
	validatedPath, err := gs.validateRepoPath(repoPath)
	if err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}

	// Validate branch name
	if err := validateBranchName(branchName); err != nil {
		return fmt.Errorf("invalid branch name: %w", err)
	}

	// Check if branch exists
	exists, err := gs.branchExists(ctx, validatedPath, branchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}

	if !exists {
		return fmt.Errorf("branch '%s' does not exist", branchName)
	}

	// Switch to branch
	if err := gs.runSafeGitCommand(ctx, validatedPath, "checkout", branchName); err != nil {
		return fmt.Errorf("failed to switch to branch: %w", err)
	}

	getLog().Info().Msgf("Successfully switched to branch '%s' in repository: %s", branchName, validatedPath)
	return nil
}

// Helper methods

// isGitRepository checks if a directory is a git repository
func (gs *GitService) isGitRepository(repoPath string) bool {
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return false
	}
	return true
}

// getCurrentBranch gets the current branch name
func (gs *GitService) getCurrentBranch(ctx context.Context, repoPath string) (string, error) {
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetCurrentCommit gets the current commit hash (public version for temporal activities)
func (gs *GitService) GetCurrentCommit(ctx context.Context, repoPath string) (string, error) {
	return gs.getCurrentCommit(ctx, repoPath)
}

// GetHeadCommitSHA gets the HEAD commit SHA (alias for GetCurrentCommit)
func (gs *GitService) GetHeadCommitSHA(ctx context.Context, repoPath string) (string, error) {
	return gs.getCurrentCommit(ctx, repoPath)
}

// getCurrentCommit gets the current commit hash
func (gs *GitService) getCurrentCommit(ctx context.Context, repoPath string) (string, error) {
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current commit: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// IsWorkingDirectoryClean checks if the working directory is clean
func (gs *GitService) IsWorkingDirectoryClean(ctx context.Context, repoPath string) (bool, error) {
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check working directory status: %w", err)
	}

	return len(strings.TrimSpace(string(output))) == 0, nil
}

// getRemoteURL gets the remote URL
func (gs *GitService) getRemoteURL(ctx context.Context, repoPath string) (string, error) {
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// hasChangesToCommit checks if there are staged changes to commit
func (gs *GitService) hasChangesToCommit(ctx context.Context, repoPath string) (bool, error) {
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "diff", "--cached", "--quiet")
	if err != nil {
		return false, fmt.Errorf("failed to build git command: %w", err)
	}

	err = cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// Exit code 1 means there are differences
			if exitError.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, fmt.Errorf("failed to check for staged changes: %w", err)
	}

	return false, nil
}

// branchExists checks if a branch exists
func (gs *GitService) branchExists(ctx context.Context, repoPath, branchName string) (bool, error) {
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "show-ref", "--verify", "--quiet", "refs/heads/"+branchName)
	if err != nil {
		return false, fmt.Errorf("failed to build git command: %w", err)
	}

	err = cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// Exit code 1 means branch doesn't exist
			if exitError.ExitCode() == 1 {
				return false, nil
			}
		}
		return false, fmt.Errorf("failed to check if branch exists: %w", err)
	}

	return true, nil
}

// GitCommit represents a git commit with its metadata
type GitCommit struct {
	Hash    string
	Message string
	Author  string
	Parents []string
}

// GetCommitHistory retrieves the commit history for the repository
func (gs *GitService) GetCommitHistory(ctx context.Context, repoPath string, limit int) ([]GitCommit, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	// Build git log command with format to extract commit info
	// Format: hash|message|author|parent_hashes
	// Use --topo-order to preserve branch topology for proper graph visualization
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "log",
		fmt.Sprintf("--max-count=%d", limit),
		"--format=%H|%s|%an|%P",
		"--all",
		"--topo-order")
	if err != nil {
		return nil, fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		// Check if it's just an empty repository
		if strings.Contains(err.Error(), "does not have any commits") {
			return []GitCommit{}, nil
		}
		return nil, fmt.Errorf("failed to get commit history: %w", err)
	}

	// Parse the output
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	commits := make([]GitCommit, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue // Skip malformed lines
		}

		commit := GitCommit{
			Hash:    parts[0],
			Message: parts[1],
			Author:  parts[2],
		}

		// Parse parent hashes (space-separated)
		if parts[3] != "" {
			commit.Parents = strings.Split(parts[3], " ")
		} else {
			commit.Parents = []string{}
		}

		commits = append(commits, commit)
	}

	return commits, nil
}

// DeleteBranch deletes a branch
func (gs *GitService) DeleteBranch(ctx context.Context, repoPath, branchName string) error {
	getLog().Debug().Msgf("Deleting branch '%s' in repository: %s", branchName, repoPath)

	// Check if branch exists
	exists, err := gs.branchExists(ctx, repoPath, branchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}

	if !exists {
		getLog().Debug().Msgf("Branch '%s' does not exist in repository: %s", branchName, repoPath)
		return nil
	}

	// Get current branch to avoid deleting current branch
	currentBranch, err := gs.getCurrentBranch(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	if currentBranch == branchName {
		return fmt.Errorf("cannot delete current branch: %s", branchName)
	}

	// Delete branch
	if err := gs.runSafeGitCommand(ctx, repoPath, "branch", "-D", branchName); err != nil {
		return fmt.Errorf("failed to delete branch: %w", err)
	}

	getLog().Info().Msgf("Successfully deleted branch '%s' in repository: %s", branchName, repoPath)
	return nil
}

// ListBranches lists all branches in the repository
func (gs *GitService) ListBranches(ctx context.Context, repoPath string) ([]string, error) {
	getLog().Debug().Msgf("Listing branches in repository: %s", repoPath)

	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "branch", "-a", "--format=%(refname:short)")
	if err != nil {
		return nil, fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	branches := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string
	for _, branch := range branches {
		branch = strings.TrimSpace(branch)
		if branch != "" && !strings.HasPrefix(branch, "origin/") {
			result = append(result, branch)
		}
	}

	return result, nil
}

// GetCommitMessage gets the commit message for a specific commit
func (gs *GitService) GetCommitMessage(ctx context.Context, repoPath, commitHash string) (string, error) {
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "log", "--format=%B", "-n", "1", commitHash)
	if err != nil {
		return "", fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit message: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetCommitAuthor gets the author information for a specific commit
func (gs *GitService) GetCommitAuthor(ctx context.Context, repoPath, commitHash string) (string, error) {
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "log", "--format=%an <%ae>", "-n", "1", commitHash)
	if err != nil {
		return "", fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit author: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetCommitTimestamp gets the timestamp for a specific commit
func (gs *GitService) GetCommitTimestamp(ctx context.Context, repoPath, commitHash string) (time.Time, error) {
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "log", "--format=%ci", "-n", "1", commitHash)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get commit timestamp: %w", err)
	}

	timestampStr := strings.TrimSpace(string(output))
	timestamp, err := time.Parse("2006-01-02 15:04:05 -0700", timestampStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse commit timestamp: %w", err)
	}

	return timestamp, nil
}

// StashChanges stashes current changes
func (gs *GitService) StashChanges(ctx context.Context, repoPath, message string) error {
	getLog().Debug().Msgf("Stashing changes in repository: %s", repoPath)

	// Check if there are changes to stash
	isClean, err := gs.IsWorkingDirectoryClean(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("failed to check working directory status: %w", err)
	}

	if isClean {
		getLog().Debug().Msgf("No changes to stash in repository: %s", repoPath)
		return nil
	}

	// Stash changes
	if err := gs.runSafeGitCommand(ctx, repoPath, "stash", "push", "-m", message); err != nil {
		return fmt.Errorf("failed to stash changes: %w", err)
	}

	getLog().Info().Msgf("Successfully stashed changes in repository: %s", repoPath)
	return nil
}

// PopStash pops the most recent stash
func (gs *GitService) PopStash(ctx context.Context, repoPath string) error {
	getLog().Debug().Msgf("Popping stash in repository: %s", repoPath)

	// Check if there are stashes
	if !gs.hasStashes(ctx, repoPath) {
		getLog().Debug().Msgf("No stashes to pop in repository: %s", repoPath)
		return nil
	}

	// Pop stash
	if err := gs.runSafeGitCommand(ctx, repoPath, "stash", "pop"); err != nil {
		return fmt.Errorf("failed to pop stash: %w", err)
	}

	getLog().Info().Msgf("Successfully popped stash in repository: %s", repoPath)
	return nil
}

// ResetToCommit resets the repository to a specific commit
func (gs *GitService) ResetToCommit(ctx context.Context, repoPath, commitHash string, hard bool) error {
	getLog().Debug().Msgf("Resetting repository to commit: %s", commitHash)

	resetType := "--mixed"
	if hard {
		resetType = "--hard"
	}

	// Reset to commit
	if err := gs.runSafeGitCommand(ctx, repoPath, "reset", resetType, commitHash); err != nil {
		return fmt.Errorf("failed to reset to commit: %w", err)
	}

	getLog().Info().Msgf("Successfully reset repository to commit: %s", commitHash)
	return nil
}

// CleanWorkingDirectory cleans untracked files and directories
func (gs *GitService) CleanWorkingDirectory(ctx context.Context, repoPath string) error {
	getLog().Debug().Msgf("Cleaning working directory: %s", repoPath)

	// Clean untracked files and directories
	if err := gs.runSafeGitCommand(ctx, repoPath, "clean", "-fd"); err != nil {
		return fmt.Errorf("failed to clean working directory: %w", err)
	}

	getLog().Info().Msgf("Successfully cleaned working directory: %s", repoPath)
	return nil
}

// hasStashes checks if there are stashes in the repository
func (gs *GitService) hasStashes(ctx context.Context, repoPath string) bool {
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "stash", "list")
	if err != nil {
		return false
	}

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return len(strings.TrimSpace(string(output))) > 0
}

// CreateBranchIfNotExists creates a branch only if it doesn't already exist
func (gs *GitService) CreateBranchIfNotExists(ctx context.Context, repoPath, branchName string) error {
	getLog().Debug().Msgf("Creating branch '%s' if not exists in repository: %s", branchName, repoPath)

	// Check if branch already exists
	exists, err := gs.branchExists(ctx, repoPath, branchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}

	if exists {
		getLog().Debug().Msgf("Branch '%s' already exists in repository: %s", branchName, repoPath)
		return nil
	}

	// Create branch
	return gs.CreateBranch(ctx, repoPath, branchName)
}

// SwitchBranchIfNotCurrent switches to a branch only if not already on it
func (gs *GitService) SwitchBranchIfNotCurrent(ctx context.Context, repoPath, branchName string) error {
	getLog().Debug().Msgf("Switching to branch '%s' if not current in repository: %s", branchName, repoPath)

	// Check current branch
	currentBranch, err := gs.getCurrentBranch(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	if currentBranch == branchName {
		getLog().Debug().Msgf("Already on branch '%s' in repository: %s", branchName, repoPath)
		return nil
	}

	// Switch branch
	return gs.SwitchBranch(ctx, repoPath, branchName)
}

// GetWorkDir returns the working directory (repository path)
func (gs *GitService) GetWorkDir() string {
	return gs.workDir
}

// Close closes the git service
func (gs *GitService) Close() error {
	getLog().Debug().Msg("Closing git service")
	return nil
}

// Helper methods for idempotent Temporal activities

// GetWorktreePath returns the expected path for a task worktree
func (gs *GitService) GetWorktreePath(taskID string) string {
	// Validate taskID
	if err := validateAgentID(taskID); err != nil {
		getLog().Debug().Msgf("Invalid task ID for worktree path: %v", err)
		return ""
	}

	// Get base path from config or use the repository path
	basePath := gs.workDir
	if gs.config != nil && gs.config.Git.WorktreeBasePath != "" {
		// Use absolute path from config
		absConfigPath, err := filepath.Abs(gs.config.Git.WorktreeBasePath)
		if err != nil {
			getLog().Debug().Msgf("Failed to get absolute path from config: %v", err)
		} else {
			basePath = absConfigPath
		}
	}

	// Build worktree path using standard naming (without timestamp for expected path)
	worktreePath := filepath.Join(basePath, ".worktrees", GenerateTaskBranchName(taskID))

	// Return absolute path
	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		getLog().Debug().Msgf("Failed to get absolute path for worktree: %v", err)
		return worktreePath
	}

	return absPath
}

// WorktreeExists checks if a worktree exists at the given path
func (gs *GitService) WorktreeExists(path string) bool {
	// Validate path
	_, err := gs.validateRepoPath(path)
	if err != nil {
		getLog().Debug().Msgf("Invalid path for worktree check: %v", err)
		return false
	}

	// Check for .git file (not directory - worktrees have a .git file)
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}

	// In worktrees, .git is a file pointing to the main repo
	return !info.IsDir()
}

// GetWorktreeBranch returns the current branch of a worktree
func (gs *GitService) GetWorktreeBranch(path string) (string, error) {
	// Validate path
	validPath, err := gs.validateRepoPath(path)
	if err != nil {
		return "", fmt.Errorf("invalid worktree path: %w", err)
	}

	// Build git command to get current branch
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd, err := gs.buildSafeGitCommand(ctx, validPath, "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("failed to build git command: %w", err)
	}

	// Execute command
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree branch: %w", err)
	}

	// Parse output
	branch := strings.TrimSpace(string(output))

	// Handle detached HEAD state
	if branch == "" {
		// Try to get commit hash for detached HEAD
		cmd, err = gs.buildSafeGitCommand(ctx, validPath, "rev-parse", "HEAD")
		if err != nil {
			return "", fmt.Errorf("failed to build git command for detached HEAD: %w", err)
		}

		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("worktree in detached HEAD state, failed to get commit: %w", err)
		}

		return fmt.Sprintf("(detached HEAD at %s)", strings.TrimSpace(string(output))[:8]), nil
	}

	return branch, nil
}

// GenerateTaskBranchName generates a consistent branch name for a task
func GenerateTaskBranchName(taskID string) string {
	return fmt.Sprintf("task-%s", taskID)
}

// GenerateTaskWorktreeName generates a consistent worktree name for a task
// Note: No timestamp to ensure idempotency - same task always gets same worktree path
func GenerateTaskWorktreeName(taskID string) string {
	return fmt.Sprintf("task-%s", taskID)
}

// ExtractTaskIDFromPath extracts task ID from a worktree path
func ExtractTaskIDFromPath(path string) string {
	// Get base name of path
	base := filepath.Base(path)

	// Handle "task-{taskID}" or "task-{taskID}-{timestamp}" patterns
	if strings.HasPrefix(base, "task-") {
		// Remove "task-" prefix
		base = strings.TrimPrefix(base, "task-")

		// Check if there's a timestamp suffix (numeric at the end)
		parts := strings.Split(base, "-")
		if len(parts) > 1 {
			// Check if the last part is numeric (timestamp)
			lastPart := parts[len(parts)-1]
			isTimestamp := true
			for _, char := range lastPart {
				if char < '0' || char > '9' {
					isTimestamp = false
					break
				}
			}

			if isTimestamp && len(lastPart) >= 8 { // Timestamp should be at least 8 digits
				// Join all parts except the last one (timestamp)
				taskID := strings.Join(parts[:len(parts)-1], "-")
				if err := validateAgentID(taskID); err == nil {
					return taskID
				}
			}
		}

		// No timestamp found with task- prefix, treat the whole thing as task ID
		if err := validateAgentID(base); err == nil {
			return base
		}
	} else {
		// Handle paths without "task-" prefix, but only if they have timestamp pattern
		parts := strings.Split(base, "-")
		if len(parts) > 1 {
			// Check if the last part is numeric (timestamp)
			lastPart := parts[len(parts)-1]
			isTimestamp := true
			for _, char := range lastPart {
				if char < '0' || char > '9' {
					isTimestamp = false
					break
				}
			}

			if isTimestamp && len(lastPart) >= 8 { // Timestamp should be at least 8 digits
				// Join all parts except the last one (timestamp)
				taskID := strings.Join(parts[:len(parts)-1], "-")
				if err := validateAgentID(taskID); err == nil {
					return taskID
				}
			}
		}
		// For paths without task- prefix and without timestamp, return empty
	}

	// No match found
	return ""
}

// AddWorktree creates a new worktree using git worktree add command
// If fromRef is a commit hash (40 or 64 hex chars), creates worktree directly from that commit.
// If fromRef is a branch name or empty, uses branch tracking behavior.
func (gs *GitService) AddWorktree(ctx context.Context, worktreePath, branchName, fromRef string) error {
	getLog().Debug().Msgf("Adding worktree at: %s, branch: %s, from: %s", worktreePath, branchName, fromRef)

	// Validate inputs
	validatedPath, err := gs.validateRepoPath(worktreePath)
	if err != nil {
		return fmt.Errorf("invalid worktree path: %w", err)
	}

	if err := validateBranchName(branchName); err != nil {
		return fmt.Errorf("invalid branch name: %w", err)
	}

	// Build command arguments
	var args []string
	if fromRef != "" {
		// Check if fromRef is a valid commit hash (40 or 64 hex chars)
		isCommitHash := (len(fromRef) == 40 || len(fromRef) == 64) && validateCommitHash(fromRef) == nil

		if isCommitHash {
			// Create worktree directly from commit without tracking
			// Use -B to reset branch if it already exists (idempotent for retries)
			args = []string{"worktree", "add", "-B", branchName, validatedPath, fromRef}
			getLog().Debug().Msgf("Creating worktree from commit SHA: %s", fromRef)
		} else {
			// fromRef is a branch name, resolve and use tracking
			resolvedBranch, err := gs.resolveToBranchName(ctx, fromRef)
			if err != nil {
				return fmt.Errorf("failed to resolve ref to branch name: %w", err)
			}

			// Create worktree with new branch from specific branch (resolved from ref)
			// Use -B instead of -b to reset the branch if it already exists (idempotent for retries)
			args = []string{"worktree", "add", "-B", branchName, "--track", validatedPath, resolvedBranch}
		}
	} else {
		// Check if branch exists
		branchExists, err := gs.branchExists(ctx, gs.workDir, branchName)
		if err != nil {
			return fmt.Errorf("failed to check if branch exists: %w", err)
		}

		if branchExists {
			// Create worktree from existing branch
			args = []string{"worktree", "add", "--track", validatedPath, branchName}
		} else {
			// Create worktree with new branch
			args = []string{"worktree", "add", "-b", branchName, "--track", validatedPath}
		}
	}

	// Execute command
	if err := gs.runSafeGitCommand(ctx, gs.workDir, args...); err != nil {
		return fmt.Errorf("failed to add worktree: %w", err)
	}

	getLog().Info().Msgf("Successfully added worktree at: %s", validatedPath)
	return nil
}

// RemoveWorktree removes a worktree using git worktree remove command
func (gs *GitService) RemoveWorktree(ctx context.Context, worktreePath string, force bool) error {
	getLog().Debug().Msgf("Removing worktree at: %s, force: %v", worktreePath, force)

	// Validate path
	validatedPath, err := gs.validateRepoPath(worktreePath)
	if err != nil {
		return fmt.Errorf("invalid worktree path: %w", err)
	}

	// Check if worktree exists
	if !gs.WorktreeExists(validatedPath) {
		getLog().Debug().Msgf("Worktree does not exist, skipping removal: %s", validatedPath)
		return nil
	}

	// Build command arguments
	args := []string{"worktree", "remove", validatedPath}
	if force {
		args = append(args, "--force")
	}

	// Execute command
	if err := gs.runSafeGitCommand(ctx, gs.workDir, args...); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	getLog().Info().Msgf("Successfully removed worktree at: %s", validatedPath)
	return nil
}

// PruneWorktrees removes stale worktree references
func (gs *GitService) PruneWorktrees(ctx context.Context) error {
	getLog().Debug().Msg("Pruning worktrees")

	if err := gs.runSafeGitCommand(ctx, gs.workDir, "worktree", "prune", "-v"); err != nil {
		return fmt.Errorf("failed to prune worktrees: %w", err)
	}

	getLog().Info().Msg("Successfully pruned worktrees")
	return nil
}

// ListWorktrees lists all worktrees using git worktree list command
func (gs *GitService) ListWorktrees(ctx context.Context) ([]string, error) {
	getLog().Debug().Msg("Listing worktrees")

	cmd, err := gs.buildSafeGitCommand(ctx, gs.workDir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			worktree := strings.TrimPrefix(line, "worktree ")
			worktrees = append(worktrees, worktree)
		}
	}

	return worktrees, nil
}

// resolveToBranchName resolves a ref (commit hash or branch name) to a branch name
// If the ref is already a branch name, it returns the branch name
// If the ref is a commit hash, it finds a branch that contains that commit
func (gs *GitService) resolveToBranchName(ctx context.Context, ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("ref cannot be empty")
	}

	// First, check if ref is already a valid branch name
	branchExists, err := gs.branchExists(ctx, gs.workDir, ref)
	if err != nil {
		return "", fmt.Errorf("failed to check if ref is a branch: %w", err)
	}

	if branchExists {
		// ref is already a branch name, return it
		return ref, nil
	}

	// ref might be a commit hash, try to find a branch that contains it
	// Use git branch --contains to find branches containing the commit
	cmd, err := gs.buildSafeGitCommand(ctx, gs.workDir, "branch", "--contains", ref, "--format=%(refname:short)")
	if err != nil {
		return "", fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to find branches containing ref %s: %w", ref, err)
	}

	branches := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(branches) == 0 || (len(branches) == 1 && branches[0] == "") {
		return "", fmt.Errorf("no branches found containing ref: %s", ref)
	}

	// Return the first branch found (typically this will be the main branch or the branch the commit belongs to)
	firstBranch := strings.TrimSpace(branches[0])
	if firstBranch == "" {
		return "", fmt.Errorf("no valid branches found containing ref: %s", ref)
	}

	getLog().Debug().Msgf("Resolved ref '%s' to branch '%s'", ref, firstBranch)
	return firstBranch, nil
}

// GetDiff returns the full git diff output for the repository
// This captures ALL changes (tracked and untracked) by first staging them
func (gs *GitService) GetDiff(ctx context.Context, repoPath string) (string, error) {
	// First, add all files to staging to capture untracked files in the diff
	// This is safe since we're capturing diff BEFORE the commit in the workflow
	addErr := gs.runSafeGitCommand(ctx, repoPath, "add", "-N", ".")
	if addErr != nil {
		// Non-critical - continue even if add fails
		getLog().Debug().Err(addErr).Msg("Failed to add files for diff capture")
	}

	// Now get the diff including staged and unstaged changes
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "diff", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		// If there's an error getting diff, check if it's because there are no commits
		if strings.Contains(err.Error(), "ambiguous argument 'HEAD'") {
			// No commits yet, return empty diff
			return "", nil
		}
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	return string(output), nil
}

// GetDiffStat returns the git diff --stat output for the repository
func (gs *GitService) GetDiffStat(ctx context.Context, repoPath string) (string, error) {
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "diff", "--stat", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		// If there's an error getting diff stat, check if it's because there are no commits
		if strings.Contains(err.Error(), "ambiguous argument 'HEAD'") {
			// No commits yet, return empty stat
			return "", nil
		}
		return "", fmt.Errorf("failed to get diff stat: %w", err)
	}

	return string(output), nil
}

// GetChangedFiles returns a list of files that have been changed
func (gs *GitService) GetChangedFiles(ctx context.Context, repoPath string) ([]string, error) {
	cmd, err := gs.buildSafeGitCommand(ctx, repoPath, "diff", "--name-only", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		// If there's an error, check if it's because there are no commits
		if strings.Contains(err.Error(), "ambiguous argument 'HEAD'") {
			// No commits yet, return empty list
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file != "" {
			result = append(result, file)
		}
	}

	return result, nil
}

// ParseDiffStat parses diff stat output to extract insertions and deletions
func (gs *GitService) ParseDiffStat(diffStat string) (insertions int, deletions int) {
	// Example diffStat format:
	// file1.go | 10 ++++++----
	// file2.go | 5 +++++
	// 2 files changed, 12 insertions(+), 3 deletions(-)

	lines := strings.Split(diffStat, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for the summary line: "X files changed, Y insertions(+), Z deletions(-)"
		if strings.Contains(line, "changed") {
			// Extract insertions
			if idx := strings.Index(line, "insertion"); idx != -1 {
				// Find the number before "insertion"
				parts := strings.Fields(line[:idx])
				if len(parts) > 0 {
					// Parse the last number before "insertion"
					if n, err := fmt.Sscanf(parts[len(parts)-1], "%d", &insertions); err == nil && n == 1 {
						// Success
					}
				}
			}

			// Extract deletions
			if idx := strings.Index(line, "deletion"); idx != -1 {
				// Find the number before "deletion"
				parts := strings.Fields(line[:idx])
				if len(parts) > 0 {
					// Parse the last number before "deletion"
					if n, err := fmt.Sscanf(parts[len(parts)-1], "%d", &deletions); err == nil && n == 1 {
						// Success
					}
				}
			}
		}
	}

	return insertions, deletions
}
