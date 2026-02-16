import type { AIActivityRecord, StepDraft, StepResult } from "../lib/types";
import { groupToolEvents } from "../lib/obs-mapping";

type Props = {
  step: StepDraft | null;
  result: StepResult | null;
  events: AIActivityRecord[];
  isOpen: boolean;
  onClose: () => void;
};

function formatTimestamp(value?: string): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleTimeString();
}

function formatTokens(n: number | undefined): string {
  if (!n) {
    return "0";
  }
  return n.toLocaleString();
}

export function StepDetailsDrawer({ step, result, events, isOpen, onClose }: Props) {
  const toolGroups = groupToolEvents(events);

  if (!isOpen || !step) {
    return null;
  }

  return (
    <aside className="details-drawer" role="complementary" aria-label="Step details">
      <header className="details-drawer__header">
        <div>
          <h3>{step.name}</h3>
          <p className="muted-text">{step.id}</p>
        </div>
        <button type="button" onClick={onClose}>
          Close
        </button>
      </header>

      {result && (
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

          {result.agent_output && (
            <details className="step-output-details">
              <summary>Agent output</summary>
              <pre className="step-output-pre">{result.agent_output}</pre>
            </details>
          )}

          {result.git_diff && (
            <details className="step-output-details">
              <summary>Git diff</summary>
              <pre className="step-output-pre">{result.git_diff}</pre>
            </details>
          )}
        </section>
      )}

      <section>
        <h4>Tool activity</h4>
        {toolGroups.length === 0 && <p className="muted-text">No tool events for this step yet.</p>}
        {toolGroups.map((group, index) => (
          <article key={`${group.toolName}-${index}`} className="tool-group-card">
            <strong>{group.toolName}</strong>
            <p>{group.input || "(no input summary)"}</p>
            {group.result ? (
              <p className={group.result.success ? "success-text" : "error-text"}>
                {group.result.success ? "Success" : "Failed"}
                {group.result.error ? `: ${group.result.error}` : ""}
              </p>
            ) : (
              <p className="muted-text">Awaiting result</p>
            )}
          </article>
        ))}
      </section>

      <section>
        <h4>Event timeline</h4>
        {events.length === 0 && <p className="muted-text">No events for this step yet.</p>}
        <ul className="event-list">
          {events.map((event) => (
            <li key={event.event_id} className="event-list__item">
              <div className="event-list__meta">
                <span>{formatTimestamp(event.timestamp)}</span>
                <span>{event.event_type}</span>
              </div>
              {event.content_preview && <p>{event.content_preview}</p>}
              {!event.content_preview && event.tool_input_summary && <p>{event.tool_input_summary}</p>}
            </li>
          ))}
        </ul>
      </section>
    </aside>
  );
}
