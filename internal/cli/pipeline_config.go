// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/protocol"

	"gopkg.in/yaml.v3"
)

// knownRuntimeVars lists variable names that are injected at pipeline runtime.
// These are substituted by injectRuntimeVars() in the pipeline workflow.
var knownRuntimeVars = []string{
	"RunID",          // Current pipeline run ID
	"StepIndex",      // Current step index (0-based)
	"StepID",         // Current step ID
	"PreviousStepID", // Previous step ID (empty for first step)
}

// PipelineFileConfig represents a pipeline YAML file
type PipelineFileConfig struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Variables   map[string]string `yaml:"variables"`
	Steps       []StepConfig      `yaml:"steps"`
	Agent       *AgentOverride    `yaml:"agent"`
}

// StepConfig defines a single step in the pipeline YAML
type StepConfig struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Prompt string `yaml:"prompt"`
}

// AgentOverride allows overriding agent settings per-pipeline
type AgentOverride struct {
	Tool    string `yaml:"tool"`
	Model   string `yaml:"model"`
	Version string `yaml:"version"`
}

// LoadPipelineFile loads and validates a pipeline YAML file
func LoadPipelineFile(path string) (*PipelineFileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read pipeline file: %w", err)
	}

	var cfg PipelineFileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse pipeline YAML: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pipeline config: %w", err)
	}

	return &cfg, nil
}

// Validate checks the pipeline config for errors
func (p *PipelineFileConfig) Validate() error {
	if p.Name == "" {
		return errors.New("pipeline name is required")
	}
	if len(p.Steps) == 0 {
		return errors.New("pipeline must have at least one step")
	}

	seenIDs := make(map[string]bool)
	for i, step := range p.Steps {
		if step.ID == "" {
			return fmt.Errorf("step %d: id is required", i+1)
		}
		if seenIDs[step.ID] {
			return fmt.Errorf("step %d: duplicate id '%s'", i+1, step.ID)
		}
		seenIDs[step.ID] = true

		if step.Prompt == "" {
			return fmt.Errorf("step %d (%s): prompt is required", i+1, step.ID)
		}
	}
	return nil
}

// ToStepInputs converts the pipeline config to protocol.StepInput slice
// cliVars override/extend variables defined in the YAML file
func (p *PipelineFileConfig) ToStepInputs(appCfg *config.AppConfig, cliVars map[string]string) ([]protocol.StepInput, error) {
	inputs := make([]protocol.StepInput, len(p.Steps))

	// Merge variables: YAML first, then CLI overrides
	mergedVars := make(map[string]string)
	for k, v := range p.Variables {
		mergedVars[k] = v
	}
	for k, v := range cliVars {
		mergedVars[k] = v // CLI takes precedence
	}

	for i, step := range p.Steps {
		// Validate that all template variables are either in mergedVars or are runtime vars
		if err := validateTemplateVars(step.Prompt, mergedVars, knownRuntimeVars); err != nil {
			return nil, fmt.Errorf("step %s: %w", step.ID, err)
		}

		// Render prompt with merged variables (preserves runtime variables like {{.RunID}})
		renderedPrompt, err := renderTemplate(step.Prompt, mergedVars)
		if err != nil {
			return nil, fmt.Errorf("step %s: failed to render prompt: %w", step.ID, err)
		}

		// Determine tool settings (pipeline override > app config)
		toolName := appCfg.Agent.DefaultTool
		toolVersion := appCfg.Agent.DefaultVersion
		if p.Agent != nil {
			if p.Agent.Tool != "" {
				toolName = p.Agent.Tool
			}
			if p.Agent.Version != "" {
				toolVersion = p.Agent.Version
			}
		}

		// Copy tool options from app config
		toolOptions := make(map[string]interface{})
		for k, v := range appCfg.Agent.ToolOptions {
			toolOptions[k] = v
		}

		// Override model if specified in pipeline
		if p.Agent != nil && p.Agent.Model != "" {
			toolOptions["model"] = p.Agent.Model
		}

		agentConfig := &protocol.AgentConfigInput{
			ToolName:       toolName,
			ToolVersion:    toolVersion,
			PromptTemplate: renderedPrompt,
			Variables:      map[string]string{}, // Already interpolated into prompt
			ToolOptions:    toolOptions,
			FlagFormat:     appCfg.Agent.FlagFormat,
		}

		inputs[i] = protocol.StepInput{
			StepID:      step.ID,
			Name:        step.Name,
			AgentConfig: agentConfig,
		}
	}

	return inputs, nil
}

// templateVarPattern matches Go template variable syntax: {{.varName}}
// Captures the variable name for selective substitution
var templateVarPattern = regexp.MustCompile(`\{\{\s*\.(\w+)\s*\}\}`)

// renderTemplate performs selective template variable substitution.
// Only replaces {{.varName}} when varName exists in vars map.
// Runtime variables (like {{.RunID}}) are preserved for later substitution.
func renderTemplate(templateStr string, vars map[string]string) (string, error) {
	result := templateVarPattern.ReplaceAllStringFunc(templateStr, func(match string) string {
		// Extract variable name from the match
		submatches := templateVarPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match // Keep original if we can't extract the name
		}

		varName := submatches[1]
		if value, exists := vars[varName]; exists {
			return value // Substitute with provided value
		}

		// Variable not in map - preserve for runtime substitution
		return match
	})

	return result, nil
}

// validateTemplateVars checks if all non-runtime variables have values provided.
// runtimeVars contains variable names that will be available at runtime.
func validateTemplateVars(templateStr string, vars map[string]string, runtimeVars []string) error {
	runtimeSet := make(map[string]bool)
	for _, rv := range runtimeVars {
		runtimeSet[rv] = true
	}

	matches := templateVarPattern.FindAllStringSubmatch(templateStr, -1)
	var missing []string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		varName := match[1]
		if _, inVars := vars[varName]; !inVars {
			if !runtimeSet[varName] {
				missing = append(missing, varName)
			}
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("undefined template variables: %s", strings.Join(missing, ", "))
	}
	return nil
}
