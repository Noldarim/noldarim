// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/pkg/containers/models"
)

// MockContainerService is a mock implementation of container service for testing
type MockContainerService struct {
	mock.Mock
}

func (m *MockContainerService) GetContainer(ctx context.Context, containerID string) (*models.Container, error) {
	args := m.Called(ctx, containerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Container), args.Error(1)
}

func (m *MockContainerService) WriteToContainer(ctx context.Context, containerID string, content string, dstPath string) error {
	args := m.Called(ctx, containerID, content, dstPath)
	return args.Error(0)
}

// Mock other required methods for the service interface (not used in our tests)
func (m *MockContainerService) CreateContainer(ctx context.Context, config models.ContainerConfig) (*models.Container, error) {
	args := m.Called(ctx, config)
	return args.Get(0).(*models.Container), args.Error(1)
}

func (m *MockContainerService) StartContainer(ctx context.Context, containerID string) error {
	args := m.Called(ctx, containerID)
	return args.Error(0)
}

func (m *MockContainerService) StopContainer(ctx context.Context, containerID string, timeout *time.Duration) error {
	args := m.Called(ctx, containerID, timeout)
	return args.Error(0)
}

func (m *MockContainerService) DeleteContainer(ctx context.Context, containerID string, force bool) error {
	args := m.Called(ctx, containerID, force)
	return args.Error(0)
}

func (m *MockContainerService) ListContainers(ctx context.Context) ([]*models.Container, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*models.Container), args.Error(1)
}

func (m *MockContainerService) RefreshContainer(ctx context.Context, containerID string) (*models.Container, error) {
	args := m.Called(ctx, containerID)
	return args.Get(0).(*models.Container), args.Error(1)
}

func (m *MockContainerService) ListContainersByLabels(ctx context.Context, labels map[string]string) ([]*models.Container, error) {
	args := m.Called(ctx, labels)
	return args.Get(0).([]*models.Container), args.Error(1)
}

func (m *MockContainerService) KillContainer(ctx context.Context, containerID string) error {
	args := m.Called(ctx, containerID)
	return args.Error(0)
}

func (m *MockContainerService) CopyFileToContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error {
	args := m.Called(ctx, containerID, srcPath, dstPath)
	return args.Error(0)
}

func (m *MockContainerService) CopyFileFromContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error {
	args := m.Called(ctx, containerID, srcPath, dstPath)
	return args.Error(0)
}

func (m *MockContainerService) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockContainerService) ExecContainer(ctx context.Context, containerID string, cmd []string, workDir string) (*models.ExecResult, error) {
	args := m.Called(ctx, containerID, cmd, workDir)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ExecResult), args.Error(1)
}

func TestAgentSetupActivities_CopyClaudeCredentialsActivity(t *testing.T) {
	cfg := &config.AppConfig{}

	tests := []struct {
		name                  string
		input                 types.CopyClaudeCredentialsActivityInput
		setupMocks            func(*MockContainerService, context.Context)
		mockKeychainResponse  string
		mockKeychainError     error
		expectedSuccess       bool
		expectedErrorContains string
		skipOnNonDarwin       bool
	}{
		{
			name: "successful_credentials_copy_on_darwin",
			input: types.CopyClaudeCredentialsActivityInput{
				ContainerID: "test-container-123",
			},
			setupMocks: func(mockService *MockContainerService, ctx context.Context) {
				mockService.On("WriteToContainer", mock.AnythingOfType("*context.valueCtx"), "test-container-123", `{"api_key": "test-key"}`, "/home/noldarim/.claude/.credentials.json").Return(nil)
			},
			mockKeychainResponse:  `{"api_key": "test-key"}`,
			mockKeychainError:     nil,
			expectedSuccess:       true,
			expectedErrorContains: "",
			skipOnNonDarwin:       true,
		},
		{
			name: "unsupported_os_error",
			input: types.CopyClaudeCredentialsActivityInput{
				ContainerID: "test-container-123",
			},
			setupMocks: func(mockService *MockContainerService, ctx context.Context) {
				// No mocks needed as it should fail before calling service
			},
			mockKeychainResponse:  "",
			mockKeychainError:     nil,
			expectedSuccess:       false,
			expectedErrorContains: "Sorry, right now only MacOS credentials obtaining for claude is supported",
			skipOnNonDarwin:       false,
		},
		{
			name: "keychain_access_fails",
			input: types.CopyClaudeCredentialsActivityInput{
				ContainerID: "test-container-123",
			},
			setupMocks: func(mockService *MockContainerService, ctx context.Context) {
				// No mocks needed since we fail at keychain level
			},
			mockKeychainResponse:  "",
			mockKeychainError:     fmt.Errorf("keychain access denied"),
			expectedSuccess:       false,
			expectedErrorContains: "failed to get credentials from keychain",
			skipOnNonDarwin:       true,
		},
		{
			name: "invalid_json_credentials",
			input: types.CopyClaudeCredentialsActivityInput{
				ContainerID: "test-container-123",
			},
			setupMocks: func(mockService *MockContainerService, ctx context.Context) {
				mockService.On("GetContainer", mock.AnythingOfType("*context.valueCtx"), "test-container-123").Return(&models.Container{
					ID:     "test-container-123",
					Status: "running",
				}, nil)
			},
			mockKeychainResponse:  "invalid json{",
			mockKeychainError:     nil,
			expectedSuccess:       false,
			expectedErrorContains: "invalid JSON credentials",
			skipOnNonDarwin:       true,
		},
		{
			name: "write_to_container_fails",
			input: types.CopyClaudeCredentialsActivityInput{
				ContainerID: "test-container-123",
			},
			setupMocks: func(mockService *MockContainerService, ctx context.Context) {
				mockService.On("WriteToContainer", mock.AnythingOfType("*context.valueCtx"), "test-container-123", `{"api_key": "test-key"}`, "/home/noldarim/.claude/.credentials.json").Return(fmt.Errorf("write failed"))
			},
			mockKeychainResponse:  `{"api_key": "test-key"}`,
			mockKeychainError:     nil,
			expectedSuccess:       false,
			expectedErrorContains: "failed to write credentials file",
			skipOnNonDarwin:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip darwin-specific tests on non-darwin systems unless explicitly testing unsupported OS
			if runtime.GOOS != "darwin" && tt.skipOnNonDarwin {
				t.Skip("Skipping Darwin-specific test on non-Darwin system")
			}

			// If testing unsupported OS on darwin, skip
			if runtime.GOOS == "darwin" && !tt.skipOnNonDarwin {
				t.Skip("Skipping unsupported OS test on Darwin system")
			}

			// Create Temporal test suite and activity environment
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()

			// Setup mock container service
			mockService := &MockContainerService{}
			tt.setupMocks(mockService, context.Background())

			// Setup mock keychain helper
			originalFunc := getClaudeCredentialsFromKeychainFunc
			getClaudeCredentialsFromKeychainFunc = func(ctx context.Context) (string, error) {
				return tt.mockKeychainResponse, tt.mockKeychainError
			}
			defer func() {
				getClaudeCredentialsFromKeychainFunc = originalFunc
			}()

			// Create activity instance
			activities := NewAgentSetupActivities(mockService, cfg)

			// Register the activity
			env.RegisterActivity(activities.CopyClaudeCredentialsActivity)

			// Execute activity
			val, err := env.ExecuteActivity(activities.CopyClaudeCredentialsActivity, tt.input)

			if tt.expectedErrorContains != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorContains)
				return
			}

			// For successful cases, get the result and verify
			assert.NoError(t, err)
			var result types.CopyClaudeCredentialsActivityOutput
			err = val.Get(&result)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedSuccess, result.Success)
			assert.Empty(t, result.Error)

			// Verify mock expectations
			mockService.AssertExpectations(t)
		})
	}
}

func TestGetClaudeCredentialsFromKeychain_ValidJSON(t *testing.T) {
	// Test that the mock system works correctly
	testJSON := `{"api_key": "test-key", "organization": "test-org"}`

	originalFunc := getClaudeCredentialsFromKeychainFunc
	getClaudeCredentialsFromKeychainFunc = func(ctx context.Context) (string, error) {
		return testJSON, nil
	}
	defer func() {
		getClaudeCredentialsFromKeychainFunc = originalFunc
	}()

	result, err := getClaudeCredentialsFromKeychain(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, testJSON, result)
	assert.True(t, json.Valid([]byte(result)))
}

func TestAgentSetupActivities_NewAgentSetupActivities(t *testing.T) {
	// Test the constructor
	mockService := &MockContainerService{}
	cfg := &config.AppConfig{}

	activities := NewAgentSetupActivities(mockService, cfg)

	assert.NotNil(t, activities)
	assert.Equal(t, cfg, activities.config)
	// We can't directly compare the interface, but we can verify it's not nil
	assert.NotNil(t, activities.containerService)
}
