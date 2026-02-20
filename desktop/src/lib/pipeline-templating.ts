// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import type { PipelineDraft, StepDraft } from "./types";

export const runtimeVariables = ["RunID", "StepIndex", "StepID", "PreviousStepID"] as const;

const templateVarPattern = /\{\{\s*\.(\w+)\s*\}\}/g;

export function validateTemplateVars(template: string, vars: Record<string, string>): string[] {
  const missing: string[] = [];
  const runtimeSet = new Set<string>(runtimeVariables);

  for (const match of template.matchAll(templateVarPattern)) {
    const variable = match[1];
    if (!variable) {
      continue;
    }
    if (!(variable in vars) && !runtimeSet.has(variable) && !missing.includes(variable)) {
      missing.push(variable);
    }
  }

  return missing;
}

export function renderTemplate(template: string, vars: Record<string, string>): string {
  return template.replace(templateVarPattern, (fullMatch, variableName: string) => {
    if (Object.prototype.hasOwnProperty.call(vars, variableName)) {
      return vars[variableName] ?? "";
    }
    return fullMatch;
  });
}

export type RenderedPipeline = {
  name: string;
  steps: StepDraft[];
};

export function renderPipelineDraft(draft: PipelineDraft): RenderedPipeline {
  const renderedSteps: StepDraft[] = draft.steps.map((step) => {
    const missing = validateTemplateVars(step.prompt, draft.variables);
    if (missing.length > 0) {
      throw new Error(`Step '${step.id}' is missing variables: ${missing.join(", ")}`);
    }

    return {
      ...step,
      prompt: renderTemplate(step.prompt, draft.variables)
    };
  });

  return {
    name: draft.name.trim(),
    steps: renderedSteps
  };
}
