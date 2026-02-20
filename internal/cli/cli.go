// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"os"
)

const (
	appName    = "noldarim"
	appVersion = "0.1.0-alpha"
)

// Execute runs the CLI application
func Execute() error {
	if len(os.Args) < 2 {
		return printUsage()
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "run":
		return runCommand(args)
	case "task":
		return taskCommand(args)
	case "diff":
		return diffCommand(args)
	case "projects":
		return projectsCommand(args)
	case "version":
		fmt.Printf("%s version %s\n", appName, appVersion)
		return nil
	case "help", "-h", "--help":
		return printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		return printUsage()
	}
}

func printUsage() error {
	fmt.Printf(`%s - AI workflow orchestrator

Usage:
  %s <command> [arguments]

Commands:
  run <task>     Run an AI task on a project
  task           Show task details (tokens, commands, diff)
  diff [run_id]  Show git diff for a pipeline run (grouped by steps and files)
  projects       List available projects
  version        Print version information
  help           Show this help message

Examples:
  %s run "Add a logout button to the header"
  %s run --project myproject "Fix the auth bug"
  %s task show abc123
  %s task show --diff
  %s diff                    # Show diff for latest run
  %s diff abc123             # Show diff for specific run
  %s projects

`, appName, appName, appName, appName, appName, appName, appName, appName, appName)
	return nil
}
