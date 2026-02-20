// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/logger"
	"github.com/noldarim/noldarim/internal/orchestrator"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui"
)

func main() {
	// Load configuration
	cfg, err := config.NewConfig("config.yaml")
	if err != nil {
		// Only log to stderr on critical startup errors before logger is initialized
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize the new logging system
	if err := logger.Initialize(&cfg.Log); err != nil {
		// Only log to stderr on critical startup errors
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.CloseGlobal()

	// Get main logger
	mainLog := logger.GetLogger("main")
	mainLog.Info().Msg("Starting noldarim application")

	// Start pprof HTTP server for memory profiling
	go func() {
		mainLog.Info().Msg("Starting pprof server on localhost:6060")
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			mainLog.Error().Err(err).Msg("pprof server failed")
		}
	}()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create channels for communication between TUI and orchestrator
	cmdChan := make(chan protocol.Command, 100)
	eventChan := make(chan protocol.Event, 100)

	// Start the orchestrator in a separate goroutine
	orch, err := orchestrator.New(cmdChan, eventChan, cfg)
	if err != nil {
		mainLog.Error().Err(err).Msg("Error creating orchestrator")
		// Log critical errors to stderr only before TUI starts
		fmt.Fprintf(os.Stderr, "Error creating orchestrator: %v\n", err)
		os.Exit(1)
	}

	// Ensure cleanup on exit
	defer func() {
		mainLog.Info().Msg("Shutting down orchestrator...")
		cancel() // Cancel context to stop orchestrator
		if err := orch.Close(); err != nil {
			mainLog.Error().Err(err).Msg("Error closing orchestrator")
			// Log to stderr since TUI might be closed
			fmt.Fprintf(os.Stderr, "Error closing orchestrator: %v\n", err)
		}
	}()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start orchestrator in background
	go func() {
		mainLog.Info().Msg("Starting orchestrator...")
		orch.Run(ctx)
		mainLog.Info().Msg("Orchestrator stopped")
	}()

	// Start TUI in background
	tuiErrChan := make(chan error, 1)
	go func() {
		mainLog.Info().Msg("Starting TUI")
		tuiErrChan <- tui.StartTUI(cmdChan, eventChan)
	}()

	// Wait for either signal or TUI to exit
	select {
	case sig := <-sigChan:
		mainLog.Info().Msgf("Received signal %v, shutting down...", sig)
		cancel() // Cancel context for graceful shutdown
	case err := <-tuiErrChan:
		if err != nil {
			mainLog.Error().Err(err).Msg("Error running TUI")
			// Log to stderr since TUI has exited
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		}
		cancel() // Cancel context when TUI exits
	}

	mainLog.Info().Msg("Application shutting down")
}
