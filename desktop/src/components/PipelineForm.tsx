import { useMemo, useRef, useState, type FormEvent } from "react";

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

type Props = {
  templates: PipelineTemplate[];
  disabled?: boolean;
  onStart: (draft: PipelineDraft) => Promise<void>;
};

export function PipelineForm({ templates, disabled, onStart }: Props) {
  const nextKeyRef = useRef(1);
  function formKey(): string {
    return `fk-${nextKeyRef.current++}`;
  }

  const [selectedTemplateId, setSelectedTemplateId] = useState<string>("");
  const [name, setName] = useState<string>("Pipeline run");
  const [variables, setVariables] = useState<VariableRow[]>([]);
  const [steps, setSteps] = useState<FormStep[]>(() => [
    { id: "step-1", name: "Step 1", prompt: "Describe what this step should do", _key: formKey() }
  ]);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);

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

    setName(template.draft.name);
    setVariables(
      Object.entries(template.draft.variables).map(([key, value]) => ({
        key,
        value
      }))
    );
    setSteps(toFormSteps(template.draft.steps, formKey));
    setError(null);
  }

  function updateStep(index: number, patch: Partial<StepDraft>) {
    setSteps((prev) => {
      const next = [...prev];
      next[index] = { ...next[index], ...patch };
      return next;
    });
  }

  function moveStep(index: number, direction: -1 | 1) {
    setSteps((prev) => {
      const target = index + direction;
      if (target < 0 || target >= prev.length) {
        return prev;
      }
      const next = [...prev];
      const [item] = next.splice(index, 1);
      next.splice(target, 0, item);
      return next;
    });
  }

  function deleteStep(index: number) {
    setSteps((prev) => prev.filter((_, currentIndex) => currentIndex !== index));
  }

  function addStep() {
    setSteps((prev) => {
      const n = prev.length + 1;
      return [
        ...prev,
        { id: `step-${n}`, name: `Step ${n}`, prompt: "", _key: formKey() }
      ];
    });
  }

  function updateVariable(index: number, patch: Partial<VariableRow>) {
    setVariables((prev) => {
      const next = [...prev];
      next[index] = { ...next[index], ...patch };
      return next;
    });
  }

  function addVariable() {
    setVariables((prev) => [...prev, { key: "", value: "" }]);
  }

  function deleteVariable(index: number) {
    setVariables((prev) => prev.filter((_, currentIndex) => currentIndex !== index));
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);

    const trimmedName = name.trim();
    if (!trimmedName) {
      setError("Pipeline name is required.");
      return;
    }
    if (steps.length === 0) {
      setError("At least one step is required.");
      return;
    }

    const validatedSteps = steps.map((step, index) => ({
      id: step.id.trim(),
      name: step.name.trim(),
      prompt: step.prompt
    }));

    for (const [index, step] of validatedSteps.entries()) {
      if (!step.id) {
        setError(`Step ${index + 1}: id is required.`);
        return;
      }
      if (!step.name) {
        setError(`Step ${index + 1}: name is required.`);
        return;
      }
      if (!step.prompt.trim()) {
        setError(`Step ${index + 1}: prompt is required.`);
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

    setIsSubmitting(true);
    try {
      await onStart({
        name: trimmedName,
        variables: variableMap,
        steps: validatedSteps
      });
    } catch (submitError) {
      const message = submitError instanceof Error ? submitError.message : "Failed to start pipeline";
      setError(message);
    } finally {
      setIsSubmitting(false);
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
              setSelectedTemplateId(value);
              if (value) {
                applyTemplate(value);
              } else {
                setError(null);
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
            onChange={(event) => setName(event.target.value)}
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
