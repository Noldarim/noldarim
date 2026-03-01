// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useState, useMemo } from "react";
import type { ToolNameGroup } from "../../lib/obs-mapping";
import type { AIActivityRecord } from "../../lib/types";
import { extractToolOutputFromPayload } from "../../lib/formatting";

type Props = { groups: ToolNameGroup[]; events?: AIActivityRecord[] };

export function ToolActivityPanel({ groups, events = [] }: Props) {
  const [expandedCalls, setExpandedCalls] = useState<Set<string>>(new Set());

  const payloadsByEventId = useMemo(() => {
    const map: Record<string, string> = {};
    for (const event of events) {
      if (event.event_type === "tool_result" && event.raw_payload) {
        map[event.event_id] = event.raw_payload;
      }
    }
    return map;
  }, [events]);

  const toggleExpand = (key: string) => {
    setExpandedCalls((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

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
          {group.calls.map((call, index) => {
            const callKey = `${group.toolName}-${index}`;
            const isExpanded = expandedCalls.has(callKey);
            const rawPayload = call.resultEventId ? payloadsByEventId[call.resultEventId] : undefined;

            let displayContent = call.result?.output;
            if (isExpanded && rawPayload) {
              const extracted = extractToolOutputFromPayload(rawPayload);
              if (extracted) displayContent = extracted;
            }

            const canExpand = rawPayload
              ? (extractToolOutputFromPayload(rawPayload)?.length ?? 0) > 200
              : (call.result?.output?.length ?? 0) > 200;

            return (
              <article key={callKey} className="tool-group-card">
                <p>{call.input || "(no input summary)"}</p>
                {call.result ? (
                  <>
                    <p className={call.result.success ? "success-text" : "error-text"}>
                      {call.result.success ? "Success" : "Failed"}
                      {call.result.error ? `: ${call.result.error}` : ""}
                    </p>

                    {displayContent && (
                      <div className="tool-output-block">{displayContent}</div>
                    )}

                    {canExpand && (
                      <button
                        className="expand-toggle"
                        onClick={() => toggleExpand(callKey)}
                      >
                        {isExpanded ? "Collapse" : "Expand"}
                      </button>
                    )}
                  </>
                ) : (
                  <p className="muted-text">Awaiting result</p>
                )}
              </article>
            );
          })}
        </details>
      ))}
    </section>
  );
}
