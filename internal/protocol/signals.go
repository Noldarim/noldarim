// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// This file defines the data structures that orchestrator can send to Agents and receive from Agents
// Importantly, this file should be used only by Orchestrator of Agents
// The main wire for this data will be Temporal therefore everything here must be serializable
// The naming convention for all of the structures here is as follows: SignalToAgent<SignalName> and SignalToOrchestrator<SignalName>
// SignalToAgent should only include data that orchestrator can send to Agent
// SignalToOrchestrator should only include data that Agent can send to orchestrator
// Signals, similarly to Commands, should be separated into Read and ReadWrite signals, however here the distinction is a bit different
// Read stuff corresponds to Temporal Queries and ReadWrite to Temporal Signals or Updates
// This is TLDR about those primitives:
// Signals: Signal to send messages asynchronously to a running Workflow, changing its state or controlling its flow in real-time.
// Queries: Query to check the progress of your Workflow or debug the internal state in real-time.
// Updates: Update to send synchronous requests to your Workflow and track it in real-time.
// Relevant doc page for temporal.io https://docs.temporal.io/encyclopedia/workflow-message-passing
package protocol

// ToAgent signals

//// Read signals

//// ReadWrite signals

// ToOrchestrator signals

//// Read signals

//// ReadWrite signals
