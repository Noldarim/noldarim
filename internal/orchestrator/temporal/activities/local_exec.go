// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
)

// HeartbeatInterval is how often we send heartbeats during command execution
const HeartbeatInterval = 5 * time.Second

// MaxHeartbeatOutputLines limits how many recent output lines we include in heartbeat
const MaxHeartbeatOutputLines = 20

// MaxOutputSize limits total captured output to prevent memory exhaustion (10MB)
const MaxOutputSize = 10 * 1024 * 1024

// ReadBufferSize is the size of the buffer for reading command output
const ReadBufferSize = 4096

// PartialLineFlushInterval is how often we flush partial lines to recent output
const PartialLineFlushInterval = 2 * time.Second

// LocalExecutionActivities provides activities that run locally using os/exec
type LocalExecutionActivities struct{}

// NewLocalExecutionActivities creates a new instance of LocalExecutionActivities
func NewLocalExecutionActivities() *LocalExecutionActivities {
	return &LocalExecutionActivities{}
}

// ExecutionProgress holds the current state of command execution for heartbeats
type ExecutionProgress struct {
	Phase          string   `json:"phase"`            // Current phase: "starting", "running", "completed"
	ElapsedSeconds float64  `json:"elapsed_seconds"`  // How long we've been running
	StdoutLines    int      `json:"stdout_lines"`     // Total stdout lines received
	StderrLines    int      `json:"stderr_lines"`     // Total stderr lines received
	RecentOutput   []string `json:"recent_output"`    // Last N lines of output (for debugging)
	Command        string   `json:"command_preview"`  // First part of command (for identification)
	Truncated      bool     `json:"truncated"`        // Whether output was truncated due to size limit
}

// outputCollector collects output from a pipe with support for:
// - Partial line buffering with periodic flush
// - Output size limiting
// - Context cancellation
// - Thread-safe access to collected data
type outputCollector struct {
	mu            sync.Mutex
	output        strings.Builder // Full output (up to MaxOutputSize)
	partialLine   bytes.Buffer    // Current incomplete line
	lineCount     int             // Number of complete lines
	totalBytes    int             // Total bytes received (even if truncated)
	truncated     bool            // Whether we hit the size limit
	recentLines   []string        // Recent lines for heartbeat
	isStderr      bool            // Whether this is stderr (for prefixing)
	lastFlushTime time.Time       // When we last flushed partial line
}

// newOutputCollector creates a new output collector
func newOutputCollector(isStderr bool) *outputCollector {
	return &outputCollector{
		recentLines:   make([]string, 0, MaxHeartbeatOutputLines),
		isStderr:      isStderr,
		lastFlushTime: time.Now(),
	}
}

// Write implements io.Writer for the collector
func (c *outputCollector) Write(p []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalBytes += len(p)

	// Process byte by byte to handle lines
	for _, b := range p {
		if b == '\n' {
			// Complete line - flush partial buffer
			line := c.partialLine.String()
			c.partialLine.Reset()
			c.lineCount++
			c.addLine(line)
			c.lastFlushTime = time.Now()
		} else {
			c.partialLine.WriteByte(b)
		}
	}

	return len(p), nil
}

// addLine adds a complete line to output and recent lines (must hold lock)
func (c *outputCollector) addLine(line string) {
	// Add to full output if under limit
	if !c.truncated {
		lineWithNewline := line + "\n"
		if c.output.Len()+len(lineWithNewline) <= MaxOutputSize {
			c.output.WriteString(lineWithNewline)
		} else {
			c.truncated = true
			c.output.WriteString("\n... OUTPUT TRUNCATED (exceeded 10MB limit) ...\n")
		}
	}

	// Always add to recent lines for visibility
	prefix := "[stdout] "
	if c.isStderr {
		prefix = "[stderr] "
	}
	truncatedLine := prefix + truncateString(line, 200)
	c.recentLines = append(c.recentLines, truncatedLine)
	if len(c.recentLines) > MaxHeartbeatOutputLines {
		c.recentLines = c.recentLines[len(c.recentLines)-MaxHeartbeatOutputLines:]
	}
}

// FlushPartial flushes any partial line that's been sitting for too long
// This ensures we see output even if there's no newline
func (c *outputCollector) FlushPartial() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.partialLine.Len() > 0 && time.Since(c.lastFlushTime) >= PartialLineFlushInterval {
		// Add partial line to recent output (but don't count as complete line)
		partial := c.partialLine.String()
		prefix := "[stdout...] "
		if c.isStderr {
			prefix = "[stderr...] "
		}
		truncatedLine := prefix + truncateString(partial, 200)
		c.recentLines = append(c.recentLines, truncatedLine)
		if len(c.recentLines) > MaxHeartbeatOutputLines {
			c.recentLines = c.recentLines[len(c.recentLines)-MaxHeartbeatOutputLines:]
		}
		c.lastFlushTime = time.Now()
	}
}

// GetStats returns current statistics (thread-safe)
func (c *outputCollector) GetStats() (lineCount int, truncated bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lineCount, c.truncated
}

// GetRecentLines returns a copy of recent lines (thread-safe)
func (c *outputCollector) GetRecentLines() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]string, len(c.recentLines))
	copy(result, c.recentLines)
	return result
}

// GetOutput returns the collected output (thread-safe)
func (c *outputCollector) GetOutput() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Include any remaining partial line
	if c.partialLine.Len() > 0 {
		return c.output.String() + c.partialLine.String()
	}
	return c.output.String()
}

// IsTruncated returns whether output was truncated
func (c *outputCollector) IsTruncated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.truncated
}

// LocalExecuteActivity executes a command locally using os/exec instead of docker exec
// It streams output and sends periodic heartbeats with progress information.
//
// Features:
// - Streams stdout/stderr with periodic heartbeats (every 5s)
// - Handles partial lines (flushes incomplete lines after 2s)
// - Limits output size to 10MB to prevent memory exhaustion
// - Respects context cancellation for clean shutdown
func (a *LocalExecutionActivities) LocalExecuteActivity(ctx context.Context, input types.LocalExecuteActivityInput) (*types.LocalExecuteActivityOutput, error) {
	logger := activity.GetLogger(ctx)

	// Log the full command for observability (including any prompts)
	commandPreview := formatCommandForLogging(input.Command)
	logger.Info("LocalExecuteActivity starting",
		"command", input.Command,
		"commandPreview", commandPreview,
		"workDir", input.WorkDir,
		"commandLength", len(input.Command))

	// Log each argument separately for very long prompts
	for i, arg := range input.Command {
		if len(arg) > 200 {
			logger.Info("Command argument (truncated)",
				"argIndex", i,
				"argLength", len(arg),
				"preview", truncateString(arg, 500))
		}
	}

	// Record initial heartbeat
	activity.RecordHeartbeat(ctx, ExecutionProgress{
		Phase:        "starting",
		Command:      commandPreview,
		RecentOutput: []string{},
	})

	// Validate command
	if len(input.Command) == 0 {
		err := fmt.Errorf("command cannot be empty")
		logger.Error("LocalExecuteActivity failed: invalid input", "error", err)
		return nil, temporal.NewApplicationError(err.Error(), "INVALID_INPUT", err)
	}

	// Create command with context for cancellation
	var cmd *exec.Cmd
	if len(input.Command) == 1 {
		cmd = exec.CommandContext(ctx, input.Command[0])
	} else {
		cmd = exec.CommandContext(ctx, input.Command[0], input.Command[1:]...)
	}

	// Set working directory if provided
	if input.WorkDir != "" {
		cmd.Dir = input.WorkDir
	}

	// Create output collectors
	stdoutCollector := newOutputCollector(false)
	stderrCollector := newOutputCollector(true)

	// Create pipes for streaming output
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		logger.Error("Failed to create stdout pipe", "error", err)
		return nil, temporal.NewApplicationError(fmt.Sprintf("failed to create stdout pipe: %v", err), "PIPE_ERROR", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		logger.Error("Failed to create stderr pipe", "error", err)
		return nil, temporal.NewApplicationError(fmt.Sprintf("failed to create stderr pipe: %v", err), "PIPE_ERROR", err)
	}

	// Start command
	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to start command", "error", err)
		return nil, temporal.NewApplicationError(fmt.Sprintf("failed to start command: %v", err), "START_ERROR", err)
	}

	logger.Info("Command started, streaming output", "pid", cmd.Process.Pid)

	// Channel to signal all goroutines to stop
	done := make(chan struct{})

	// WaitGroup for output readers
	var wg sync.WaitGroup
	wg.Add(2)

	// copyWithContext copies from reader to writer, respecting context cancellation
	copyWithContext := func(dst io.Writer, src io.Reader, name string) {
		defer wg.Done()
		buf := make([]byte, ReadBufferSize)
		for {
			select {
			case <-ctx.Done():
				logger.Info("Context cancelled, stopping reader", "reader", name)
				return
			case <-done:
				// Drain any remaining data after done signal
				io.Copy(dst, src)
				return
			default:
				n, err := src.Read(buf)
				if n > 0 {
					dst.Write(buf[:n])
				}
				if err != nil {
					if err != io.EOF {
						logger.Warn("Error reading from pipe", "reader", name, "error", err)
					}
					return
				}
			}
		}
	}

	// Stream stdout and stderr
	go copyWithContext(stdoutCollector, stdoutPipe, "stdout")
	go copyWithContext(stderrCollector, stderrPipe, "stderr")

	// Heartbeat goroutine - sends progress updates and flushes partial lines
	heartbeatDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(HeartbeatInterval)
		partialFlushTicker := time.NewTicker(PartialLineFlushInterval)
		defer ticker.Stop()
		defer partialFlushTicker.Stop()

		for {
			select {
			case <-partialFlushTicker.C:
				// Flush partial lines so we see output even without newlines
				stdoutCollector.FlushPartial()
				stderrCollector.FlushPartial()

			case <-ticker.C:
				stdoutLines, stdoutTruncated := stdoutCollector.GetStats()
				stderrLines, stderrTruncated := stderrCollector.GetStats()

				// Combine recent lines from both collectors
				recentOutput := mergeRecentLines(
					stdoutCollector.GetRecentLines(),
					stderrCollector.GetRecentLines(),
				)

				elapsed := time.Since(startTime).Seconds()

				progress := ExecutionProgress{
					Phase:          "running",
					ElapsedSeconds: elapsed,
					StdoutLines:    stdoutLines,
					StderrLines:    stderrLines,
					RecentOutput:   recentOutput,
					Command:        commandPreview,
					Truncated:      stdoutTruncated || stderrTruncated,
				}

				activity.RecordHeartbeat(ctx, progress)
				logger.Info("Heartbeat: command still running",
					"elapsed", fmt.Sprintf("%.1fs", elapsed),
					"stdoutLines", stdoutLines,
					"stderrLines", stderrLines,
					"truncated", progress.Truncated)

			case <-ctx.Done():
				logger.Info("Context cancelled, stopping heartbeat")
				return

			case <-heartbeatDone:
				return
			}
		}
	}()

	// Wait for command to finish (this also closes the pipes)
	runErr := cmd.Wait()

	// Signal done to readers so they drain remaining data
	close(done)

	// Wait for output readers to finish
	wg.Wait()

	// Stop heartbeat goroutine
	close(heartbeatDone)

	duration := time.Since(startTime)

	// Process execution result
	exitCode := 0
	var errorMsg string

	if runErr != nil {
		if exitError, ok := runErr.(*exec.ExitError); ok {
			// Command ran but exited with non-zero status
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
			errorMsg = fmt.Sprintf("command exited with code %d", exitCode)
		} else if ctx.Err() != nil {
			// Context was cancelled
			errorMsg = fmt.Sprintf("command cancelled: %v", ctx.Err())
			exitCode = -1
			logger.Info("Command was cancelled", "error", ctx.Err())
		} else {
			// Command failed to run
			errorMsg = runErr.Error()
			exitCode = -1
			logger.Error("Command execution failed",
				"error", runErr,
				"command", input.Command,
				"workDir", input.WorkDir)
			return nil, temporal.NewApplicationError(
				fmt.Sprintf("command execution failed: %v", runErr),
				"EXECUTION_ERROR",
				runErr,
			)
		}
	}

	// Get final stats
	stdoutLines, stdoutTruncated := stdoutCollector.GetStats()
	stderrLines, stderrTruncated := stderrCollector.GetStats()

	result := &types.LocalExecuteActivityOutput{
		ExitCode:    exitCode,
		Output:      stdoutCollector.GetOutput(),
		ErrorOutput: stderrCollector.GetOutput(),
		Duration:    duration,
		Success:     exitCode == 0,
		Error:       errorMsg,
	}

	// Final heartbeat with completion status
	recentOutput := mergeRecentLines(
		stdoutCollector.GetRecentLines(),
		stderrCollector.GetRecentLines(),
	)

	activity.RecordHeartbeat(ctx, ExecutionProgress{
		Phase:          "completed",
		ElapsedSeconds: duration.Seconds(),
		StdoutLines:    stdoutLines,
		StderrLines:    stderrLines,
		RecentOutput:   recentOutput,
		Command:        commandPreview,
		Truncated:      stdoutTruncated || stderrTruncated,
	})

	// Log with appropriate level based on success
	if result.Success {
		logger.Info("Command execution completed successfully",
			"exitCode", exitCode,
			"duration", duration,
			"stdoutLines", stdoutLines,
			"stderrLines", stderrLines,
			"outputLength", len(result.Output),
			"truncated", stdoutTruncated || stderrTruncated)
	} else {
		logger.Error("Command execution failed: non-zero exit code",
			"exitCode", exitCode,
			"duration", duration,
			"stdoutLines", stdoutLines,
			"stderrLines", stderrLines,
			"stderrPreview", truncateString(result.ErrorOutput, 500),
			"error", errorMsg,
			"command", input.Command,
			"workDir", input.WorkDir,
			"truncated", stdoutTruncated || stderrTruncated)
	}

	return result, nil
}

// mergeRecentLines combines recent lines from stdout and stderr,
// keeping the most recent MaxHeartbeatOutputLines total
func mergeRecentLines(stdout, stderr []string) []string {
	combined := make([]string, 0, len(stdout)+len(stderr))
	combined = append(combined, stdout...)
	combined = append(combined, stderr...)

	// Keep only the last N lines
	if len(combined) > MaxHeartbeatOutputLines {
		combined = combined[len(combined)-MaxHeartbeatOutputLines:]
	}
	return combined
}

// formatCommandForLogging creates a short preview of the command for logging
func formatCommandForLogging(command []string) string {
	if len(command) == 0 {
		return "<empty>"
	}

	// For Claude commands, show the tool name and a preview of the prompt
	preview := command[0]
	if len(command) > 1 {
		// Add first few args
		for i := 1; i < len(command) && i < 4; i++ {
			arg := command[i]
			if len(arg) > 50 {
				arg = arg[:50] + "..."
			}
			preview += " " + arg
		}
		if len(command) > 4 {
			preview += fmt.Sprintf(" [+%d more args]", len(command)-4)
		}
	}

	return truncateString(preview, 200)
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
