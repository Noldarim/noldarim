// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

type Props = { output: string; defaultOpen?: boolean };

export function AgentOutput({ output, defaultOpen = false }: Props) {
  if (!output.trim()) return null;
  return (
    <details className="step-output-details" open={defaultOpen || undefined}>
      <summary>Agent output</summary>
      <pre className="step-output-pre">{output}</pre>
    </details>
  );
}
