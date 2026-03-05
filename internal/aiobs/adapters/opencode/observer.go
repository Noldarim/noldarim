// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/noldarim/noldarim/internal/aiobs/types"
)

// defaultSSEURL is the default OpenCode SSE endpoint.
const defaultSSEURL = "http://localhost:3000/event"

// envOpenCodeURL is the environment variable to override the SSE URL.
const envOpenCodeURL = "OPENCODE_URL"

type Observer struct{}

func NewObserver() *Observer {
	return &Observer{}
}

func (o *Observer) RuntimeName() string {
	return "opencode"
}

func (o *Observer) Discover(ctx context.Context, run types.RunContext) (types.DiscoverySpec, error) {
	_ = ctx

	// Determine SSE URL: env var > WorkDir > default
	sseURL := defaultSSEURL
	if envURL := os.Getenv(envOpenCodeURL); envURL != "" {
		sseURL = envURL
	} else if run.WorkDir != "" {
		// WorkDir could contain a URL override in the future
		_ = run.WorkDir
	}

	return types.DiscoverySpec{
		Streams: []types.StreamSpec{
			{
				Name: "opencode-events",
				Type: "sse",
				Root: sseURL,
				Glob: "",
			},
		},
	}, nil
}

func (o *Observer) NewParser(run types.RunContext) types.Parser {
	return &Parser{
		run:               run,
		seenSessions:      make(map[string]bool),
		sequenceBySession: make(map[string]int64),
	}
}

type Parser struct {
	run               types.RunContext
	mu                sync.Mutex
	seenSessions      map[string]bool
	sequenceBySession map[string]int64
}

// openCodeEvent represents the raw SSE event structure from OpenCode.
type openCodeEvent struct {
	Type       string          `json:"type"`
	Properties json.RawMessage `json:"properties"`
}

// openCodeProperties are common fields in the properties object.
type openCodeProperties struct {
	SessionID string `json:"sessionId"`
	ID        string `json:"id"`
	Content   string `json:"content"`
	Message   string `json:"message"`
	Error     string `json:"error"`
	Model     string `json:"model"`
}

func (p *Parser) OnLine(ctx context.Context, stream types.StreamID, line []byte) ([]types.ParsedEvent, error) {
	_ = ctx

	p.mu.Lock()
	defer p.mu.Unlock()

	var rawEvent openCodeEvent
	if err := json.Unmarshal(line, &rawEvent); err != nil {
		return nil, nil // Skip unparseable events
	}

	// Extract properties
	var props openCodeProperties
	if rawEvent.Properties != nil {
		_ = json.Unmarshal(rawEvent.Properties, &props)
	}

	// Extract session ID: try sessionId first, then id
	sessionID := props.SessionID
	if sessionID == "" {
		sessionID = props.ID
	}

	var events []types.ParsedEvent

	switch rawEvent.Type {
	case "session.created":
		event := types.ParsedEvent{
			EventID:        generateEventID(),
			SessionID:      sessionID,
			EventType:      types.EventTypeSessionStart,
			Kind:           types.KindLifecycle,
			Level:          types.LevelInfo,
			Timestamp:      time.Now(),
			SourceFile:     stream.Name,
			ContentPreview: "OpenCode session created",
			RawPayload:     line,
		}
		events = append(events, event)

	case "session.updated":
		contentPreview := props.Content
		if contentPreview == "" {
			contentPreview = props.Message
		}
		if len(contentPreview) > 500 {
			contentPreview = contentPreview[:500]
		}
		event := types.ParsedEvent{
			EventID:        generateEventID(),
			SessionID:      sessionID,
			EventType:      types.EventTypeAIOutput,
			Kind:           types.KindMessage,
			Level:          types.LevelInfo,
			Timestamp:      time.Now(),
			SourceFile:     stream.Name,
			ContentPreview: contentPreview,
			ContentLength:  len(props.Content),
			Model:          props.Model,
			RawPayload:     line,
		}
		events = append(events, event)

	case "session.completed":
		event := types.ParsedEvent{
			EventID:        generateEventID(),
			SessionID:      sessionID,
			EventType:      types.EventTypeSessionEnd,
			Kind:           types.KindLifecycle,
			Level:          types.LevelInfo,
			Timestamp:      time.Now(),
			SourceFile:     stream.Name,
			ContentPreview: "OpenCode session completed",
			RawPayload:     line,
		}
		events = append(events, event)

	case "session.error":
		errorMsg := props.Error
		if errorMsg == "" {
			errorMsg = props.Message
		}
		event := types.ParsedEvent{
			EventID:        generateEventID(),
			SessionID:      sessionID,
			EventType:      types.EventTypeError,
			Kind:           types.KindError,
			Level:          types.LevelError,
			Timestamp:      time.Now(),
			SourceFile:     stream.Name,
			ContentPreview: errorMsg,
			ToolError:      errorMsg,
			RawPayload:     line,
		}
		events = append(events, event)

	case "tool.start":
		var toolProps struct {
			Name  string `json:"name"`
			Input string `json:"input"`
		}
		if rawEvent.Properties != nil {
			_ = json.Unmarshal(rawEvent.Properties, &toolProps)
		}
		event := types.ParsedEvent{
			EventID:          generateEventID(),
			SessionID:        sessionID,
			EventType:        types.EventTypeToolUse,
			Kind:             types.KindTool,
			Level:            types.LevelInfo,
			Timestamp:        time.Now(),
			SourceFile:       stream.Name,
			ToolName:         toolProps.Name,
			ToolInputSummary: toolProps.Input,
			RawPayload:       line,
		}
		events = append(events, event)

	case "tool.result":
		var toolProps struct {
			Name    string `json:"name"`
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		if rawEvent.Properties != nil {
			_ = json.Unmarshal(rawEvent.Properties, &toolProps)
		}
		success := toolProps.Success
		event := types.ParsedEvent{
			EventID:     generateEventID(),
			SessionID:   sessionID,
			EventType:   types.EventTypeToolResult,
			Kind:        types.KindTool,
			Level:       types.LevelInfo,
			Timestamp:   time.Now(),
			SourceFile:  stream.Name,
			ToolName:    toolProps.Name,
			ToolSuccess: &success,
			ToolError:   toolProps.Error,
			RawPayload:  line,
		}
		events = append(events, event)
	}

	// Assign session IDs and sequences
	for i := range events {
		if events[i].SessionID != "" && !p.seenSessions[events[i].SessionID] {
			p.seenSessions[events[i].SessionID] = true
		}
		events[i].Sequence = p.nextSequence(events[i].SessionID)
	}

	return events, nil
}

func (p *Parser) Flush(ctx context.Context) ([]types.ParsedEvent, error) {
	_ = ctx
	return nil, nil
}

func (p *Parser) nextSequence(sessionID string) int64 {
	if sessionID == "" {
		sessionID = "_default"
	}
	p.sequenceBySession[sessionID]++
	return p.sequenceBySession[sessionID]
}

var eventCounter uint32
var eventCounterMu sync.Mutex

func generateEventID() string {
	eventCounterMu.Lock()
	eventCounter++
	count := eventCounter
	eventCounterMu.Unlock()
	return fmt.Sprintf("oc-%s-%04x", time.Now().Format("20060102150405.000000000"), count&0xFFFF)
}
