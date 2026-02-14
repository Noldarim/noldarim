// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
)

type projectsOptions struct {
	configPath string
}

func projectsCommand(args []string) error {
	opts := &projectsOptions{}
	fs := flag.NewFlagSet("projects", flag.ExitOnError)
	fs.StringVar(&opts.configPath, "config", "config.yaml", "Path to config file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return listProjects(opts)
}

func listProjects(opts *projectsOptions) error {
	// Load configuration
	cfg, err := config.NewConfig(opts.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create data service (just DB access, no orchestrator)
	dataService, err := services.NewDataService(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer dataService.Close()

	// Query projects
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	projects, err := dataService.LoadProjects(ctx)
	if err != nil {
		return fmt.Errorf("failed to load projects: %w", err)
	}

	if len(projects) == 0 {
		fmt.Println("No projects found.")
		fmt.Println("\nCreate a project using the TUI app:")
		fmt.Println("  make run")
		return nil
	}

	// Print table
	fmt.Println()
	fmt.Printf("%-20s  %-40s  %s\n", "NAME", "ID", "PATH")
	fmt.Println("────────────────────  ────────────────────────────────────────  ────────────────────────────────")
	for _, p := range projects {
		name := p.Name
		if len(name) > 20 {
			name = name[:17] + "..."
		}
		id := p.ID
		if len(id) > 40 {
			id = id[:37] + "..."
		}
		fmt.Printf("%-20s  %-40s  %s\n", name, id, p.RepositoryPath)
	}
	fmt.Println()

	return nil
}
