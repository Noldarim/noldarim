import { describe, expect, it } from "vitest";

import { renderPipelineDraft, renderTemplate, validateTemplateVars } from "./pipeline-templating";

describe("pipeline templating", () => {
  it("renders provided variables and preserves runtime variables", () => {
    const rendered = renderTemplate(
      "Implement {{.feature}} and read from {{.RunID}} + {{.StepIndex}}",
      { feature: "search" }
    );

    expect(rendered).toContain("Implement search");
    expect(rendered).toContain("{{.RunID}}");
    expect(rendered).toContain("{{.StepIndex}}");
  });

  it("reports missing non-runtime variables", () => {
    const missing = validateTemplateVars("Fix {{.feature}} in {{.language}}", { feature: "auth" });
    expect(missing).toEqual(["language"]);
  });

  it("fails fast when draft contains undefined variables", () => {
    expect(() =>
      renderPipelineDraft({
        name: "My pipeline",
        variables: {},
        steps: [
          {
            id: "step-1",
            name: "Step 1",
            prompt: "Implement {{.missing_var}}"
          }
        ]
      })
    ).toThrowError(/missing variables: missing_var/i);
  });
});
