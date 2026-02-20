// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useMemo, useReducer, useRef, type FormEvent } from "react";

import type { PipelineDraft, StepDraft } from "../lib/types";
import type { PipelineTemplate } from "../lib/templates";

type VariableRow = {
  key: string;
  value: string;
};

/** Internal form step with an immutable key for React reconciliation. */
type FormStep = StepDraft & { _key: string };

function toFormSteps(steps: StepDraft[], keyFn: () => string): FormStep[] {
  return steps.map((step) => ({ ...step, _key: keyFn() }));
}

type FormState = {
  selectedTemplateId: string;
  name: string;
  variables: VariableRow[];
  steps: FormStep[];
  error: string | null;
  isSubmitting: boolean;
};

type FormAction =
  | Partial<FormState>
  | ((prev: FormState) => Partial<FormState>);

function formReducer(state: FormState, action: FormAction): FormState {
  const patch = typeof action === "function" ? action(state) : action;
  return { ...state, ...patch };
}

type Props = {
  templates: PipelineTemplate[];
  disabled?: boolean;
  onStart: (draft: PipelineDraft) => Promise<void>;
};

export function PipelineForm({ templates, disabled, onStart }: Props) {
  function formKey(): string {
    return crypto.randomUUID();
  }

  const stepCounter = useRef(1);
  function nextStepId(): string {
    return `step-${++stepCounter.current}`;
  }

  const [state, dispatch] = useReducer(formReducer, null, () => ({
    selectedTemplateId: "",
    name: "Pipeline run",
    variables: [] as VariableRow[],
    steps: [{ id: "step-1", name: "Step 1", prompt: "Describe what this step should do", _key: formKey() }],
    error: null,
    isSubmitting: false
  }));

  const { selectedTemplateId, name, variables, steps, error, isSubmitting } = state;

  const templateLookup = useMemo(() => {
    const lookup = new Map<string, PipelineTemplate>();
    for (const template of templates) {
      lookup.set(template.id, template);
    }
    return lookup;
  }, [templates]);

  function applyTemplate(templateId: string) {
    const template = templateLookup.get(templateId);
    if (!template) {
      return;
    }

    dispatch({
      selectedTemplateId: templateId,
      name: template.draft.name,
      variables: Object.entries(template.draft.variables).map(([key, value]) => ({ key, value })),
      steps: toFormSteps(template.draft.steps, formKey),
      error: null
    });
  }

  function updateStep(index: number, patch: Partial<StepDraft>) {
    dispatch((prev) => {
      const next = [...prev.steps];
      next[index] = { ...next[index], ...patch };
      return { steps: next };
    });
  }

  function moveStep(index: number, direction: -1 | 1) {
    dispatch((prev) => {
      const target = index + direction;
      if (target < 0 || target >= prev.steps.length) {
        return {};
      }
      const next = [...prev.steps];
      const [item] = next.splice(index, 1);
      next.splice(target, 0, item);
      return { steps: next };
    });
  }

  function deleteStep(index: number) {
    dispatch((prev) => ({ steps: prev.steps.filter((_, i) => i !== index) }));
  }

  function addStep() {
    const id = nextStepId();
    dispatch((prev) => ({
      steps: [...prev.steps, { id, name: `Step ${prev.steps.length + 1}`, prompt: "", _key: formKey() }]
    }));
  }

  function updateVariable(index: number, patch: Partial<VariableRow>) {
    dispatch((prev) => {
      const next = [...prev.variables];
      next[index] = { ...next[index], ...patch };
      return { variables: next };
    });
  }

  function addVariable() {
    dispatch((prev) => ({ variables: [...prev.variables, { key: "", value: "" }] }));
  }

  function deleteVariable(index: number) {
    dispatch((prev) => ({ variables: prev.variables.filter((_, i) => i !== index) }));
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    dispatch({ error: null });

    const trimmedName = name.trim();
    if (!trimmedName) {
      dispatch({ error: "Pipeline name is required." });
      return;
    }
    if (steps.length === 0) {
      dispatch({ error: "At least one step is required." });
      return;
    }

    const validatedSteps = steps.map((step, index) => ({
      id: step.id.trim(),
      name: step.name.trim(),
      prompt: step.prompt
    }));

    for (const [index, step] of validatedSteps.entries()) {
      if (!step.id) {
        dispatch({ error: `Step ${index + 1}: id is required.` });
        return;
      }
      if (!step.name) {
        dispatch({ error: `Step ${index + 1}: name is required.` });
        return;
      }
      if (!step.prompt.trim()) {
        dispatch({ error: `Step ${index + 1}: prompt is required.` });
        return;
      }
    }

    const variableMap: Record<string, string> = {};
    for (const row of variables) {
      const key = row.key.trim();
      if (!key) {
        continue;
      }
      variableMap[key] = row.value;
    }

    dispatch({ isSubmitting: true });
    try {
      await onStart({
        name: trimmedName,
        variables: variableMap,
        steps: validatedSteps
      });
    } catch (submitError) {
      const message = submitError instanceof Error ? submitError.message : "Failed to start pipeline";
      dispatch({ error: message });
    } finally {
      dispatch({ isSubmitting: false });
    }
  }

  return (
    <section className="panel pipeline-form">
      <h2>Pipeline</h2>
      <form onSubmit={handleSubmit}>
        <div className="field-group">
          <label htmlFor="pipeline-template">Template</label>
          <select
            id="pipeline-template"
            value={selectedTemplateId}
            onChange={(event) => {
              const value = event.target.value;
              if (value) {
                applyTemplate(value);
              } else {
                dispatch({ selectedTemplateId: "", error: null });
              }
            }}
            disabled={disabled || isSubmitting}
          >
            <option value="">Custom pipeline</option>
            {templates.map((template) => (
              <option key={template.id} value={template.id}>
                {template.name}
              </option>
            ))}
          </select>
        </div>

        <div className="field-group">
          <label htmlFor="pipeline-name">Run Name</label>
          <input
            id="pipeline-name"
            value={name}
            onChange={(event) => dispatch({ name: event.target.value })}
            disabled={disabled || isSubmitting}
          />
        </div>

        <div className="field-group">
          <div className="section-header-inline">
            <label>Variables</label>
            <button type="button" onClick={addVariable} disabled={disabled || isSubmitting}>
              Add variable
            </button>
          </div>
          {variables.length === 0 && <p className="muted-text">No variables defined.</p>}
          {variables.map((row, index) => (
            <div key={`var-${index}`} className="row-inline">
              <input
                placeholder="key"
                value={row.key}
                onChange={(event) => updateVariable(index, { key: event.target.value })}
                disabled={disabled || isSubmitting}
              />
              <input
                placeholder="value"
                value={row.value}
                onChange={(event) => updateVariable(index, { value: event.target.value })}
                disabled={disabled || isSubmitting}
              />
              <button
                type="button"
                onClick={() => deleteVariable(index)}
                disabled={disabled || isSubmitting}
                className="danger-link"
              >
                Remove
              </button>
            </div>
          ))}
        </div>

        <div className="field-group">
          <div className="section-header-inline">
            <label>Steps</label>
            <button type="button" onClick={addStep} disabled={disabled || isSubmitting}>
              Add step
            </button>
          </div>
          <div className="step-list">
            {steps.map((step, index) => (
              <article key={step._key} className="step-card">
                <div className="step-card-header">
                  <strong>Step {index + 1}</strong>
                  <div className="step-actions">
                    <button type="button" onClick={() => moveStep(index, -1)} disabled={index === 0 || disabled || isSubmitting}>
                      Up
                    </button>
                    <button
                      type="button"
                      onClick={() => moveStep(index, 1)}
                      disabled={index === steps.length - 1 || disabled || isSubmitting}
                    >
                      Down
                    </button>
                    <button
                      type="button"
                      onClick={() => deleteStep(index)}
                      disabled={steps.length === 1 || disabled || isSubmitting}
                      className="danger-link"
                    >
                      Delete
                    </button>
                  </div>
                </div>
                <input
                  placeholder="step_id"
                  value={step.id}
                  onChange={(event) => updateStep(index, { id: event.target.value })}
                  disabled={disabled || isSubmitting}
                />
                <input
                  placeholder="Step name"
                  value={step.name}
                  onChange={(event) => updateStep(index, { name: event.target.value })}
                  disabled={disabled || isSubmitting}
                />
                <textarea
                  placeholder="Step prompt"
                  value={step.prompt}
                  onChange={(event) => updateStep(index, { prompt: event.target.value })}
                  rows={5}
                  disabled={disabled || isSubmitting}
                />
              </article>
            ))}
          </div>
        </div>

        <button type="submit" disabled={disabled || isSubmitting} className="primary-button">
          {isSubmitting ? "Starting..." : "Start Pipeline"}
        </button>

        {error && <p className="error-text">{error}</p>}
      </form>
    </section>
  );
}
