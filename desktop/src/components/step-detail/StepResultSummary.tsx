// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import type { StepResult } from "../../lib/types";
import { formatTokens } from "../../lib/formatting";

type Props = { result: StepResult };

export function StepResultSummary({ result }: Props) {
  return (
    <section>
      <h4>Step result</h4>
      {result.error_message && (
        <p className="error-text">{result.error_message}</p>
      )}

      <dl className="step-result-grid">
        {result.commit_sha && (
          <>
            <dt>Commit</dt>
            <dd><code>{result.commit_sha.slice(0, 10)}</code> {result.commit_message}</dd>
          </>
        )}
        <dt>Files changed</dt>
        <dd>{result.files_changed} (+{result.insertions} -{result.deletions})</dd>
        <dt>Tokens</dt>
        <dd>
          {formatTokens(result.input_tokens)} in / {formatTokens(result.output_tokens)} out
          {(result.cache_read_tokens > 0 || result.cache_create_tokens > 0) &&
            ` (cache: ${formatTokens(result.cache_read_tokens)} read, ${formatTokens(result.cache_create_tokens)} create)`}
        </dd>
        {result.duration > 0 && (
          <>
            <dt>Duration</dt>
            <dd>{(result.duration / 1e9).toFixed(1)}s</dd>
          </>
        )}
      </dl>
    </section>
  );
}
