// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/screens/taskdetails"
)

// devModel wraps the screen only to add scenario switching and mock events
type devModel struct {
	screen       taskdetails.Model
	currentScen  string
	scenarios    map[string]models.Task
	width        int
	height       int
	eventIndex   int
	mockRecords  []*models.AIActivityRecord
}

func (m devModel) Init() tea.Cmd {
	return m.screen.Init()
}

// tickMsg is used to simulate streaming events
type tickMsg struct{}

func tickCmd() tea.Msg {
	time.Sleep(500 * time.Millisecond)
	return tickMsg{}
}

func (m devModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle window size changes
	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = sizeMsg.Width
		m.height = sizeMsg.Height
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		// Dev-only: Switch scenarios with letter keys (a/b/c)
		case "a", "b", "c":
			scenKey := map[string]string{"a": "1", "b": "2", "c": "3"}[msg.String()]
			m.currentScen = scenKey
			task := m.scenarios[scenKey]
			m.screen = taskdetails.NewModel(&task, task.ProjectID, make(chan protocol.Command, 1))
			m.screen.SetSize(m.width, m.height)
			m.eventIndex = 0
			return m, nil

		// Inject a mock AI activity record
		case "e":
			if m.eventIndex < len(m.mockRecords) {
				record := m.mockRecords[m.eventIndex]
				m.eventIndex++
				// Set TaskID and ProjectID on the record
				record.TaskID = m.scenarios[m.currentScen].ID
				// Send record directly (implements common.Event)
				updated, cmd := m.screen.Update(record)
				m.screen = updated.(taskdetails.Model)
				return m, cmd
			}
			return m, nil

		// Start streaming simulation
		case "s":
			m.eventIndex = 0
			// Send stream start
			startEvent := protocol.AIStreamStartEvent{
				TaskID:    m.scenarios[m.currentScen].ID,
				ProjectID: m.scenarios[m.currentScen].ProjectID,
			}
			updated, _ := m.screen.Update(startEvent)
			m.screen = updated.(taskdetails.Model)
			return m, tickCmd

		// End streaming
		case "x":
			endEvent := protocol.AIStreamEndEvent{
				TaskID:      m.scenarios[m.currentScen].ID,
				ProjectID:   m.scenarios[m.currentScen].ProjectID,
				TotalEvents: m.eventIndex,
				FinalStatus: "completed",
			}
			updated, cmd := m.screen.Update(endEvent)
			m.screen = updated.(taskdetails.Model)
			return m, cmd
		}

	case tickMsg:
		// Auto-inject next record during streaming
		if m.eventIndex < len(m.mockRecords) {
			record := m.mockRecords[m.eventIndex]
			m.eventIndex++
			// Set TaskID on the record
			record.TaskID = m.scenarios[m.currentScen].ID
			// Send record directly (implements common.Event)
			updated, _ := m.screen.Update(record)
			m.screen = updated.(taskdetails.Model)
			return m, tickCmd
		}
		return m, nil
	}

	// Forward everything else to the screen
	updated, cmd := m.screen.Update(msg)
	m.screen = updated.(taskdetails.Model)
	return m, cmd
}

func (m devModel) View() string {
	help := "\n[a/b/c] Switch scenario  [1/2/3] Switch tab  [e] Add event  [s] Start stream  [x] End stream\n\n"
	return help + m.screen.View()
}

func main() {
	// Create mock scenarios
	scenarios := map[string]models.Task{
		"1": createTaskWithSmallDiff(),
		"2": createTaskWithLargeDiff(),
		"3": createTaskWithMergeConflict(),
	}

	// Start with scenario 1
	currentScen := "1"
	task := scenarios[currentScen]
	cmdChan := make(chan protocol.Command, 1)

	// Create the screen
	screen := taskdetails.NewModel(&task, task.ProjectID, cmdChan)
	screen.SetSize(100, 40)

	// Create mock AI activity records
	mockRecords := createMockAIRecords(task.ID)

	// Wrap only for dev scenario switching
	model := devModel{
		screen:      screen,
		currentScen: currentScen,
		scenarios:   scenarios,
		width:       100,
		height:      40,
		mockRecords: mockRecords,
	}

	fmt.Println("Task Details Screen - Dev Mode")
	fmt.Println("Press a/b/c to switch scenarios, 1/2/3 to switch tabs")
	fmt.Println("Press 'e' to inject mock event, 's' to start streaming, 'x' to end")
	fmt.Println("Press Ctrl+C to quit")
	fmt.Println("")

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

// createMockAIRecords creates a sequence of mock AI activity records
func createMockAIRecords(taskID string) []*models.AIActivityRecord {
	baseTime := time.Now()
	records := make([]*models.AIActivityRecord, 0)
	trueVal := true

	// Session start
	records = append(records, &models.AIActivityRecord{
		EventID:   "evt-001",
		TaskID:    taskID,
		SessionID: "session-abc123",
		Timestamp: baseTime,
		EventType: models.AIEventSessionStart,
	})

	// Tool calls sequence
	toolCalls := []struct {
		name  string
		input string
	}{
		{"Bash", "ls -la /workspace"},
		{"Read", "/workspace/main.go"},
		{"Grep", "func main"},
		{"Read", "/workspace/config.yaml"},
		{"Edit", "/workspace/main.go"},
		{"Bash", "go build ./..."},
		{"Bash", "go test ./..."},
	}

	for i, tc := range toolCalls {
		// Tool use
		records = append(records, &models.AIActivityRecord{
			EventID:          fmt.Sprintf("evt-%03d", i*2+2),
			TaskID:           taskID,
			SessionID:        "session-abc123",
			Timestamp:        baseTime.Add(time.Duration(i*2+1) * time.Second),
			EventType:        models.AIEventToolUse,
			ToolName:         tc.name,
			ToolInputSummary: tc.input,
		})

		// Tool result
		records = append(records, &models.AIActivityRecord{
			EventID:        fmt.Sprintf("evt-%03d", i*2+3),
			TaskID:         taskID,
			SessionID:      "session-abc123",
			Timestamp:      baseTime.Add(time.Duration(i*2+2) * time.Second),
			EventType:      models.AIEventToolResult,
			ToolName:       tc.name,
			ToolSuccess:    &trueVal,
			ContentPreview: "Command executed successfully",
		})
	}

	// Session end
	records = append(records, &models.AIActivityRecord{
		EventID:      "evt-final",
		TaskID:       taskID,
		SessionID:    "session-abc123",
		Timestamp:    baseTime.Add(20 * time.Second),
		EventType:    models.AIEventSessionEnd,
		StopReason:   "completed",
		InputTokens:  5420,
		OutputTokens: 10000,
	})

	return records
}

// Mock data scenarios for testing different views

func createTaskWithSmallDiff() models.Task {
	now := time.Now()
	return models.Task{
		ID:        "task-1",
		ProjectID: "proj-1",
		Title:     "Fix login button styling",
		Description: `Quick CSS fix for the login button on the homepage.

## Changes
- Update button padding
- Fix hover state color
- Add border radius`,
		Status:        models.TaskStatusInProgress,
		CreatedAt:     now.Add(-2 * time.Hour),
		LastUpdatedAt: now.Add(-15 * time.Minute),
		GitDiff: `diff --git a/styles/login.css b/styles/login.css
index 1234567..abcdefg 100644
--- a/styles/login.css
+++ b/styles/login.css
@@ -10,3 +10,4 @@
 .login-btn {
-  padding: 8px;
+  padding: 12px 24px;
+  border-radius: 4px;
 }`,
	}
}

func createTaskWithLargeDiff() models.Task {
	now := time.Now()
	return models.Task{
		ID:        "task-2",
		ProjectID: "proj-1",
		Title:     "Implement OAuth2 Authentication",
		Description: `## Overview
Implement OAuth2 authentication flow for the application to support third-party login providers.

## Requirements
- Support Google OAuth2
- Support GitHub OAuth2
- Implement proper token storage
- Add refresh token handling

## Technical Details
The implementation should follow OAuth2 best practices:
1. Use PKCE for additional security
2. Store tokens securely (encrypted at rest)
3. Implement proper token refresh logic
4. Handle edge cases (expired tokens, revoked access)

## Acceptance Criteria
- [ ] Users can login with Google
- [ ] Users can login with GitHub
- [ ] Tokens are refreshed automatically
- [ ] Proper error handling for auth failures
- [ ] Security audit passed

## Notes
Consider using an established OAuth2 library rather than implementing from scratch.
Coordinate with the security team for review before deployment.`,
		Status:        models.TaskStatusInProgress,
		CreatedAt:     now.Add(-72 * time.Hour),
		LastUpdatedAt: now.Add(-30 * time.Minute),
		GitDiff: `diff --git a/internal/auth/oauth.go b/internal/auth/oauth.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/internal/auth/oauth.go
@@ -0,0 +1,50 @@
+package auth
+
+import (
+    "crypto/rand"
+    "encoding/base64"
+    "golang.org/x/oauth2"
+    "golang.org/x/oauth2/google"
+    "golang.org/x/oauth2/github"
+)
+
+// OAuthProvider represents a configured OAuth provider
+type OAuthProvider struct {
+    ClientID     string
+    ClientSecret string
+    RedirectURL  string
+    Scopes       []string
+    Config       *oauth2.Config
+}
+
+// NewGoogleProvider creates a Google OAuth provider
+func NewGoogleProvider(clientID, clientSecret, redirectURL string) *OAuthProvider {
+    config := &oauth2.Config{
+        ClientID:     clientID,
+        ClientSecret: clientSecret,
+        RedirectURL:  redirectURL,
+        Scopes:       []string{"profile", "email"},
+        Endpoint:     google.Endpoint,
+    }
+    return &OAuthProvider{Config: config}
+}
+
+// NewGitHubProvider creates a GitHub OAuth provider
+func NewGitHubProvider(clientID, clientSecret, redirectURL string) *OAuthProvider {
+    config := &oauth2.Config{
+        ClientID:     clientID,
+        ClientSecret: clientSecret,
+        RedirectURL:  redirectURL,
+        Scopes:       []string{"user:email"},
+        Endpoint:     github.Endpoint,
+    }
+    return &OAuthProvider{Config: config}
+}
+
+// GenerateState generates a random state string for CSRF protection
+func GenerateState() (string, error) {
+    b := make([]byte, 32)
+    _, err := rand.Read(b)
+    if err != nil {
+        return "", err
+    }
+    return base64.URLEncoding.EncodeToString(b), nil
+}`,
	}
}

func createTaskWithMergeConflict() models.Task {
	now := time.Now()
	return models.Task{
		ID:        "task-3",
		ProjectID: "proj-1",
		Title:     "Merge feature-auth into main",
		Description: `Merge the authentication feature branch into main branch.

## Conflicts to Resolve
- config/app.yaml (version numbers)
- internal/server/routes.go (route definitions)

## Notes
Review conflicts carefully before resolving.`,
		Status:        models.TaskStatusPending,
		CreatedAt:     now.Add(-24 * time.Hour),
		LastUpdatedAt: now.Add(-5 * time.Minute),
		GitDiff: `diff --git a/config/app.yaml b/config/app.yaml
index abc1234..def5678 100644
--- a/config/app.yaml
+++ b/config/app.yaml
@@@ -1,5 -1,5 +1,9 @@@
 version: 1.2.0
+<<<<<<< HEAD
+version: 1.2.1
+=======
+version: 1.3.0
+>>>>>>> feature-auth
 port: 8080

diff --git a/internal/server/routes.go b/internal/server/routes.go
index def5678..ghi9012 100644
--- a/internal/server/routes.go
+++ b/internal/server/routes.go
@@@ -10,8 +10,13 @@@
 func RegisterRoutes(r *mux.Router) {
     r.HandleFunc("/api/health", healthHandler)
+<<<<<<< HEAD
+    r.HandleFunc("/api/users", usersHandler)
+    r.HandleFunc("/api/tasks", tasksHandler)
+=======
     r.HandleFunc("/api/login", loginHandler)
     r.HandleFunc("/api/logout", logoutHandler)
+    r.HandleFunc("/api/refresh", refreshHandler)
+>>>>>>> feature-auth
 }`,
	}
}
