// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"

	"github.com/noldarim/noldarim/internal/aiobs/types"
)

// agentFileRegex matches sub-agent transcript files like "agent-abc123.jsonl".
var agentFileRegex = regexp.MustCompile(`^agent-([a-zA-Z0-9]+)\.jsonl$`)

type ClaudeObserver struct{}

func NewObserver() *ClaudeObserver {
	return &ClaudeObserver{}
}

func (o *ClaudeObserver) RuntimeName() string {
	return "claude"
}

func (o *ClaudeObserver) Discover(ctx context.Context, run types.RunContext) (types.DiscoverySpec, error) {
	_ = ctx
	_ = run

	return types.DiscoverySpec{
		Streams: []types.StreamSpec{
			{
				Name: "claude-transcripts",
				Type: "fs-jsonl",
				Root: "/home/noldarim/.claude/projects",
				Glob: "**/*.jsonl",
			},
		},
	}, nil
}

func (o *ClaudeObserver) NewParser(run types.RunContext) types.Parser {
	return &ClaudeParser{
		run:               run,
		adapter:           &Adapter{},
		toolUseNames:      make(map[string]string),
		seenSessions:      make(map[string]bool),
		sequenceBySession: make(map[string]int64),
	}
}

type ClaudeParser struct {
	run               types.RunContext
	adapter           *Adapter
	mu                sync.Mutex
	toolUseNames      map[string]string
	seenSessions      map[string]bool
	sequenceBySession map[string]int64
	mainSessionID     string // First session ID seen from a non-agent file
}

func (p *ClaudeParser) OnLine(ctx context.Context, stream types.StreamID, line []byte) ([]types.ParsedEvent, error) {
	_ = ctx

	p.mu.Lock()
	defer p.mu.Unlock()

	entry := TranscriptEntry{}
	if err := json.Unmarshal(line, &entry); err != nil {
		return nil, fmt.Errorf("claude parser: failed to decode transcript entry: %w", err)
	}

	raw := types.RawEntry{
		Line:      0,
		Data:      json.RawMessage(line),
		SessionID: types.ExtractSessionID(json.RawMessage(line)),
	}

	events, err := p.adapter.ParseEntry(raw)
	if err != nil {
		return nil, fmt.Errorf("claude parser: %w", err)
	}

	if entry.Message != nil {
		for _, item := range entry.Message.Content {
			if item.Type == "tool_use" && item.ID != "" && item.Name != "" {
				p.toolUseNames[item.ID] = item.Name
			}
		}

	}

	var toolResultIDs []string
	if entry.Message != nil {
		toolResultIDs = make([]string, 0, len(entry.Message.Content))
		for _, item := range entry.Message.Content {
			if item.Type == "tool_result" && item.ToolUseID != "" {
				toolResultIDs = append(toolResultIDs, item.ToolUseID)
			}
		}
	}

	// Determine if this stream is from an agent file
	isAgentFile := agentFileRegex.MatchString(stream.Name)

	result := make([]types.ParsedEvent, 0, len(events)+1)
	toolResultIndex := 0

	for i := range events {
		event := events[i]
		event.SourceFile = stream.Name

		if event.SessionID != "" {
			// Track main session ID from non-agent files
			if !isAgentFile && p.mainSessionID == "" {
				p.mainSessionID = event.SessionID
			}

			if !p.seenSessions[event.SessionID] {
				p.seenSessions[event.SessionID] = true
				sessionStart := types.ParsedEvent{
					EventID:        generateEventID(),
					SessionID:      event.SessionID,
					EventType:      types.EventTypeSessionStart,
					Kind:           types.KindLifecycle,
					Level:          types.LevelInfo,
					Timestamp:      event.Timestamp,
					Sequence:       p.nextSequence(event.SessionID),
					SourceFile:     stream.Name,
					ContentPreview: fmt.Sprintf("Session started (%s)", stream.Name),
				}
				// Set ParentSessionID on agent file session_start events
				if isAgentFile && p.mainSessionID != "" {
					sessionStart.ParentSessionID = p.mainSessionID
					sessionStart.IsSidechain = true
				}
				result = append(result, sessionStart)
			}
			event.Sequence = p.nextSequence(event.SessionID)
		}

		// Set ParentSessionID on all events from agent files
		if isAgentFile && p.mainSessionID != "" {
			event.ParentSessionID = p.mainSessionID
			event.IsSidechain = true
		}

		if event.EventType == types.EventTypeToolResult && event.ToolName == "" {
			if toolResultIndex < len(toolResultIDs) {
				if name, ok := p.toolUseNames[toolResultIDs[toolResultIndex]]; ok {
					event.ToolName = name
				}
				toolResultIndex++
			} else if entry.ToolUseResult != nil {
				var probe struct {
					ToolUseID string `json:"tool_use_id"`
				}
				if json.Unmarshal(entry.ToolUseResult, &probe) == nil {
					if name, ok := p.toolUseNames[probe.ToolUseID]; ok {
						event.ToolName = name
					}
				}
			}
		}

		result = append(result, event)
	}

	return result, nil
}

func (p *ClaudeParser) Flush(ctx context.Context) ([]types.ParsedEvent, error) {
	_ = ctx

	return nil, nil
}

func (p *ClaudeParser) nextSequence(sessionID string) int64 {
	p.sequenceBySession[sessionID]++
	return p.sequenceBySession[sessionID]
}
