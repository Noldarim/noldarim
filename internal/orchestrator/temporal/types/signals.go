// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package types

// Signal names for AIObservabilityWorkflow communication.
// Consolidated here to avoid duplication between activities and workflows packages.
const (
	// RawTranscriptLineSignal is the signal for individual raw transcript lines from the watcher.
	// Deprecated: Use RawTranscriptBatchSignal for efficiency.
	RawTranscriptLineSignal = "raw-transcript-line"

	// RawTranscriptBatchSignal is the signal for batched raw transcript lines from the watcher.
	// Deprecated: Use ParsedTranscriptBatchSignal when RuntimeName is set.
	RawTranscriptBatchSignal = "raw-transcript-batch"

	// ParsedTranscriptBatchSignal is the signal for batches of already-parsed events.
	// Sent by WatchTranscriptActivity when using the Observer/Parser pipeline.
	ParsedTranscriptBatchSignal = "parsed-transcript-batch"

	// StepChangeSignal is sent by PipelineWorkflow to communicate the current step ID.
	StepChangeSignal = "step-change"
)
