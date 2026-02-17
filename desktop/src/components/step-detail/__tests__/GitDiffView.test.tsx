import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("diff2html/lib/ui/js/diff2html-ui-slim.js", () => {
  class Diff2HtmlUI {
    private target: HTMLElement;
    private diff: string;
    constructor(target: HTMLElement, diff: string) {
      this.target = target;
      this.diff = diff;
    }
    draw() {
      this.target.innerHTML = `<div class="d2h-wrapper">mocked:${this.diff.length}</div>`;
    }
    highlightCode() {}
  }
  return { Diff2HtmlUI };
});

import { GitDiffView } from "../GitDiffView";

describe("GitDiffView", () => {
  it("renders within a collapsible details element", () => {
    render(<GitDiffView diff="--- a/foo\n+++ b/foo\n@@ -1 +1 @@\n-old\n+new" />);
    expect(screen.getByText("Git diff")).toBeInTheDocument();
  });

  it("renders diff2html output in a div", () => {
    const { container } = render(<GitDiffView diff="some diff" />);
    const diffDiv = container.querySelector(".diff-view");
    expect(diffDiv).not.toBeNull();
    expect(diffDiv?.innerHTML).toContain("d2h-wrapper");
  });
});
