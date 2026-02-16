// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/logger"
	"github.com/noldarim/noldarim/internal/orchestrator"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/server"
)

func main() {
	cfg, err := config.NewConfig("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Initialize(&cfg.Log); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.CloseGlobal()

	mainLog := logger.GetLogger("main")
	mainLog.Info().Msg("Starting noldarim API server")

	// This context drives the orchestrator's lifetime.
	ctx, cancel := context.WithCancel(context.Background())

	// cmdChan is required by orchestrator.New() but the server never writes to
	// it — all mutations go through PipelineService directly. The orchestrator's
	// Run() loop blocks on this channel, which is fine: it just idles.
	cmdChan := make(chan protocol.Command, 100)
	eventChan := make(chan protocol.Event, 100)

	orch, err := orchestrator.New(cmdChan, eventChan, cfg)
	if err != nil {
		mainLog.Error().Err(err).Msg("Error creating orchestrator")
		fmt.Fprintf(os.Stderr, "Error creating orchestrator: %v\n", err)
		os.Exit(1)
	}

	// Start orchestrator
	go func() {
		mainLog.Info().Msg("Starting orchestrator...")
		orch.Run(ctx)
		mainLog.Info().Msg("Orchestrator stopped")
	}()

	// Start API server
	srv := server.New(
		&cfg.Server,
		eventChan,
		orch.DataService(),
		orch.GitServiceManager(),
		orch.PipelineService(),
		server.AgentDefaultsResponse{
			ToolName:    cfg.Agent.DefaultTool,
			ToolVersion: cfg.Agent.DefaultVersion,
			FlagFormat:  cfg.Agent.FlagFormat,
			ToolOptions: cfg.Agent.ToolOptions,
		},
	)

	serverErrChan := make(chan error, 1)
	go func() {
		serverErrChan <- srv.Run(ctx)
	}()

	// Wait for signal or server error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		mainLog.Info().Msgf("Received signal %v, shutting down...", sig)
	case err := <-serverErrChan:
		if err != nil {
			mainLog.Error().Err(err).Msg("Server error")
		}
	}

	// Graceful shutdown: fresh context with timeout, independent of orchestrator ctx.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		mainLog.Error().Err(err).Msg("Error shutting down server")
	}

	// Now stop the orchestrator
	mainLog.Info().Msg("Shutting down orchestrator...")
	cancel()
	if err := orch.Close(); err != nil {
		mainLog.Error().Err(err).Msg("Error closing orchestrator")
	}

	mainLog.Info().Msg("API server shut down")
}
