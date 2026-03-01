// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useEffect, useRef } from "react";
import { Diff2HtmlUI } from "diff2html/lib/ui/js/diff2html-ui-slim.js";
import "diff2html/bundles/css/diff2html.min.css";

type Props = { diff: string; inline?: boolean; open?: boolean };

export function GitDiffView({ diff, inline = false, open }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    const ui = new Diff2HtmlUI(el, diff, {
      drawFileList: false,
      outputFormat: "line-by-line",
      matching: "lines",
      highlight: true
    });
    ui.draw();
    ui.highlightCode();

    return () => {
      el.innerHTML = "";
    };
  }, [diff]);

  const diffContainer = <div className="diff-view" ref={containerRef} />;

  if (inline) return diffContainer;

  return (
    <details className="step-output-details" open={open || undefined}>
      <summary>Git diff</summary>
      {diffContainer}
    </details>
  );
}
