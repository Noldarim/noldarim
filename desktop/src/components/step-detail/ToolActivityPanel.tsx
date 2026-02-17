import type { ToolNameGroup } from "../../lib/obs-mapping";

type Props = { groups: ToolNameGroup[] };

export function ToolActivityPanel({ groups }: Props) {
  return (
    <section>
      <h4>Tool activity</h4>
      {groups.length === 0 && <p className="muted-text">No tool events for this step yet.</p>}
      {groups.map((group) => (
        <details key={group.toolName} className="tool-name-group">
          <summary>
            {group.toolName}
            <span className="tool-count-badge">{group.calls.length}</span>
          </summary>
          {group.calls.map((call, index) => (
            <article key={`${group.toolName}-${index}`} className="tool-group-card">
              <p>{call.input || "(no input summary)"}</p>
              {call.result ? (
                <p className={call.result.success ? "success-text" : "error-text"}>
                  {call.result.success ? "Success" : "Failed"}
                  {call.result.error ? `: ${call.result.error}` : ""}
                </p>
              ) : (
                <p className="muted-text">Awaiting result</p>
              )}
            </article>
          ))}
        </details>
      ))}
    </section>
  );
}
