import { useEffect, useRef } from "react";
import { Diff2HtmlUI } from "diff2html/lib/ui/js/diff2html-ui-slim.js";
import "diff2html/bundles/css/diff2html.min.css";

type Props = { diff: string };

export function GitDiffView({ diff }: Props) {
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

  return (
    <details className="step-output-details">
      <summary>Git diff</summary>
      <div className="diff-view" ref={containerRef} />
    </details>
  );
}
