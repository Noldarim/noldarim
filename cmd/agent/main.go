// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/noldarim/noldarim/internal/aiobs/adapters"
	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/logger"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/activities"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/workflows"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Initialize basic logger for agent
	if err := logger.Initialize(&config.LogConfig{
		Level:  "INFO",
		Format: "console",
		Output: []config.LogOutputConfig{{Type: "console", Enabled: true}},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := logger.CloseGlobal(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing logger: %v\n", err)
		}
	}()

	agentLog := logger.GetLogger("agent")
	agentLog.Info().Msg("Starting Temporal agent")

	// Register AI adapters for transcript parsing
	adapters.RegisterAll()
	agentLog.Info().Strs("adapters", adapters.RegisteredAdapters()).Msg("Registered AI adapters")

	// Get environment variables
	hostPort := os.Getenv("TEMPORAL_HOST_PORT")
	namespace := os.Getenv("TEMPORAL_NAMESPACE")
	taskQueue := os.Getenv("TEMPORAL_TASK_QUEUE")
	taskID := os.Getenv("TASK_ID")

	if hostPort == "" || namespace == "" || taskQueue == "" {
		agentLog.Fatal().
			Str("hostPort", hostPort).
			Str("namespace", namespace).
			Str("taskQueue", taskQueue).
			Msg("Missing required environment variables")
	}

	agentLog.Info().
		Str("hostPort", hostPort).
		Str("namespace", namespace).
		Str("taskQueue", taskQueue).
		Str("taskID", taskID).
		Msg("Starting agent with configuration")

	// Create Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort:  hostPort,
		Namespace: namespace,
		Logger:    logger.GetTemporalLogAdapter("temporal-agent"),
	})
	if err != nil {
		agentLog.Fatal().Err(err).Msg("Failed to create Temporal client")
	}
	defer temporalClient.Close()

	// Create worker
	w := worker.New(temporalClient, taskQueue, worker.Options{
		Identity:                               taskID,
		MaxConcurrentActivityExecutionSize:     10,
		MaxConcurrentWorkflowTaskExecutionSize: 10,
	})

	// Register workflows
	w.RegisterWorkflow(workflows.ProcessTaskWorkflow)
	w.RegisterWorkflow(workflows.AIObservabilityWorkflow)
	w.RegisterWorkflow(workflows.ProcessingStepWorkflow) // For pipeline execution

	// Register activities - agent needs execution and observability activities
	localExecActivities := activities.NewLocalExecutionActivities()
	w.RegisterActivity(localExecActivities.LocalExecuteActivity)

	// Register PrepareAgentCommand function directly as PrepareAgentCommandActivity
	w.RegisterActivityWithOptions(activities.PrepareAgentCommand, activity.RegisterOptions{
		Name: "PrepareAgentCommandActivity",
	})

	// Register transcript watcher activities for AI observability
	transcriptWatcherActivities := activities.NewTranscriptWatcherActivities(temporalClient)
	w.RegisterActivity(transcriptWatcherActivities.WatchTranscriptActivity)

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start worker in background
	go func() {
		agentLog.Info().Msg("Starting Temporal worker")
		if err := w.Run(worker.InterruptCh()); err != nil {
			agentLog.Error().Err(err).Msg("Worker stopped with error")
		}
	}()

	agentLog.Info().Msg("Agent started successfully, waiting for workflows...")

	// Wait for shutdown signal
	<-sigChan
	agentLog.Info().Msg("Shutdown signal received, stopping worker...")

	// Stop worker gracefully
	w.Stop()
	agentLog.Info().Msg("Agent shutdown complete")
}
