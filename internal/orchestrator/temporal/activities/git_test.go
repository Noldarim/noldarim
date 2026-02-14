// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestCaptureGitDiffActivity_WithTemporalTestEnvironment(t *testing.T) {
	tests := []struct {
		name          string
		setupRepo     func(t *testing.T, repoPath string)
		expectedError string
		expectDiff    bool
		expectChanges bool
	}{
		{
			name: "successful git diff capture with changes",
			setupRepo: func(t *testing.T, repoPath string) {
				// Create and modify a file
				testFile := filepath.Join(repoPath, "test.txt")
				err := os.WriteFile(testFile, []byte("test content\n"), 0644)
				require.NoError(t, err)
			},
			expectDiff:    true,
			expectChanges: true,
		},
		{
			name: "git diff with no changes",
			setupRepo: func(t *testing.T, repoPath string) {
				// Don't modify anything - clean working directory
			},
			expectDiff:    false,
			expectChanges: false,
		},
		{
			name: "git diff with multiple file changes",
			setupRepo: func(t *testing.T, repoPath string) {
				// Create multiple files
				files := []string{"file1.txt", "file2.txt", "file3.txt"}
				for _, file := range files {
					testFile := filepath.Join(repoPath, file)
					err := os.WriteFile(testFile, []byte("content for "+file+"\n"), 0644)
					require.NoError(t, err)
				}
			},
			expectDiff:    true,
			expectChanges: true,
		},
		{
			name: "invalid repository path",
			setupRepo: func(t *testing.T, repoPath string) {
				// This test will use an invalid path
			},
			expectedError: "repository path must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create Temporal test suite and activity environment
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()

			// Create temporary test repository
			tmpDir := t.TempDir()
			repoPath := filepath.Join(tmpDir, "test-repo")

			// Only create repo if not testing invalid path
			if tt.expectedError != "repository path must be provided" {
				// Create git service with test repository
				cfg := &config.AppConfig{
					Git: config.GitConfig{
						WorktreeBasePath: tmpDir,
					},
				}

				gitService, err := services.NewGitServiceWithConfig(repoPath, cfg, true)
				require.NoError(t, err)
				defer gitService.Close()

				// Set up repository state
				if tt.setupRepo != nil {
					tt.setupRepo(t, repoPath)
				}

				// Create git service manager
				gitServiceManager := services.NewGitServiceManager(cfg)

				// Create git activities
				gitActivities := NewGitActivities(gitServiceManager)

				// Register the activity
				env.RegisterActivity(gitActivities.CaptureGitDiffActivity)

				// Execute the activity
				input := types.CaptureGitDiffActivityInput{
					RepositoryPath: repoPath,
				}

				val, err := env.ExecuteActivity(gitActivities.CaptureGitDiffActivity, input)

				if tt.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.expectedError)
					return
				}

				require.NoError(t, err)

				// Verify the result
				var result types.CaptureGitDiffActivityOutput
				err = val.Get(&result)
				require.NoError(t, err)

				assert.True(t, result.Success)
				assert.Empty(t, result.Error)

				if tt.expectDiff {
					assert.NotEmpty(t, result.Diff, "Expected diff output to be present")
					assert.NotEmpty(t, result.DiffStat, "Expected diff stat to be present")
				}

				if tt.expectChanges {
					assert.True(t, result.HasChanges, "Expected HasChanges to be true")
					assert.NotEmpty(t, result.FilesChanged, "Expected FilesChanged to have entries")
					// For added files, we expect insertions
					assert.Greater(t, result.Insertions, 0, "Expected insertions count to be greater than 0")
				} else {
					assert.False(t, result.HasChanges, "Expected HasChanges to be false")
					assert.Empty(t, result.FilesChanged, "Expected FilesChanged to be empty")
					assert.Equal(t, 0, result.Insertions, "Expected 0 insertions")
					assert.Equal(t, 0, result.Deletions, "Expected 0 deletions")
				}
			} else {
				// Test with invalid path
				cfg := &config.AppConfig{}
				gitServiceManager := services.NewGitServiceManager(cfg)
				gitActivities := NewGitActivities(gitServiceManager)
				env.RegisterActivity(gitActivities.CaptureGitDiffActivity)

				input := types.CaptureGitDiffActivityInput{
					RepositoryPath: "", // Invalid empty path
				}

				_, err := env.ExecuteActivity(gitActivities.CaptureGitDiffActivity, input)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

func TestCaptureGitDiffActivity_DiffParsing(t *testing.T) {
	// Create Temporal test suite and activity environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	// Create temporary test repository
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	// Create git service with test repository
	cfg := &config.AppConfig{
		Git: config.GitConfig{
			WorktreeBasePath: tmpDir,
		},
	}

	gitService, err := services.NewGitServiceWithConfig(repoPath, cfg, true)
	require.NoError(t, err)
	defer gitService.Close()

	// Create a file with known content
	testFile := filepath.Join(repoPath, "test.txt")
	content := "line 1\nline 2\nline 3\n"
	err = os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Create git service manager and activities
	gitServiceManager := services.NewGitServiceManager(cfg)
	gitActivities := NewGitActivities(gitServiceManager)
	env.RegisterActivity(gitActivities.CaptureGitDiffActivity)

	// Execute the activity
	input := types.CaptureGitDiffActivityInput{
		RepositoryPath: repoPath,
	}

	val, err := env.ExecuteActivity(gitActivities.CaptureGitDiffActivity, input)
	require.NoError(t, err)

	var result types.CaptureGitDiffActivityOutput
	err = val.Get(&result)
	require.NoError(t, err)

	// Verify the diff contains expected patterns
	assert.Contains(t, result.Diff, "test.txt", "Diff should mention the changed file")
	assert.Contains(t, result.FilesChanged, "test.txt", "FilesChanged should include test.txt")
	assert.Equal(t, 1, len(result.FilesChanged), "Should have exactly 1 changed file")

	// Verify insertions (3 lines added)
	assert.Equal(t, 3, result.Insertions, "Should have 3 insertions")
	assert.Equal(t, 0, result.Deletions, "Should have 0 deletions")
}

func TestCaptureGitDiffActivity_WithDeletions(t *testing.T) {
	// Create Temporal test suite and activity environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	// Create temporary test repository
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	// Create git service with test repository
	cfg := &config.AppConfig{
		Git: config.GitConfig{
			WorktreeBasePath: tmpDir,
		},
	}

	gitService, err := services.NewGitServiceWithConfig(repoPath, cfg, true)
	require.NoError(t, err)
	defer gitService.Close()

	// Create and commit a file first
	testFile := filepath.Join(repoPath, "delete-test.txt")
	err = os.WriteFile(testFile, []byte("line 1\nline 2\nline 3\n"), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	err = gitService.CreateCommit(ctx, repoPath, "Add file to be deleted")
	require.NoError(t, err)

	// Now delete the file
	err = os.Remove(testFile)
	require.NoError(t, err)

	// Create git service manager and activities
	gitServiceManager := services.NewGitServiceManager(cfg)
	gitActivities := NewGitActivities(gitServiceManager)
	env.RegisterActivity(gitActivities.CaptureGitDiffActivity)

	// Execute the activity
	input := types.CaptureGitDiffActivityInput{
		RepositoryPath: repoPath,
	}

	val, err := env.ExecuteActivity(gitActivities.CaptureGitDiffActivity, input)
	require.NoError(t, err)

	var result types.CaptureGitDiffActivityOutput
	err = val.Get(&result)
	require.NoError(t, err)

	// Verify deletions are captured
	assert.True(t, result.HasChanges)
	assert.Contains(t, result.FilesChanged, "delete-test.txt")
	assert.Equal(t, 0, result.Insertions, "Should have 0 insertions")
	assert.Equal(t, 3, result.Deletions, "Should have 3 deletions")
}
