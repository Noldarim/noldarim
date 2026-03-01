// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useState } from "react";
import type { AIActivityRecord } from "../../lib/types";
import { formatTimestamp, extractToolOutputFromPayload } from "../../lib/formatting";

type Props = { events: AIActivityRecord[] };

function compareEventsByTimestamp(a: AIActivityRecord, b: AIActivityRecord): number {
  const aTime = Date.parse(a.timestamp);
  const bTime = Date.parse(b.timestamp);
  const aValid = !Number.isNaN(aTime);
  const bValid = !Number.isNaN(bTime);

  if (aValid && bValid && aTime !== bTime) {
    return aTime - bTime;
  }
  if (aValid && !bValid) {
    return -1;
  }
  if (!aValid && bValid) {
    return 1;
  }
  return a.event_id.localeCompare(b.event_id);
}

export function EventTimeline({ events }: Props) {
  const sortedEvents = [...events].sort(compareEventsByTimestamp);
  const [expandedEvents, setExpandedEvents] = useState<Set<string>>(new Set());

  const toggleExpand = (eventId: string) => {
    setExpandedEvents((prev) => {
      const next = new Set(prev);
      if (next.has(eventId)) {
        next.delete(eventId);
      } else {
        next.add(eventId);
      }
      return next;
    });
  };

  return (
    <section>
      <h4>Event timeline</h4>
      {events.length === 0 && <p className="muted-text">No events for this step yet.</p>}
      <ul className="event-list">
        {sortedEvents.map((event) => {
          const isTruncated = event.content_preview?.endsWith("...") && !!event.raw_payload;
          const isExpanded = expandedEvents.has(event.event_id);
          const fullContent = isExpanded ? extractToolOutputFromPayload(event.raw_payload) : null;

          return (
            <li key={event.event_id} className="event-list__item">
              <div className="event-list__meta">
                <span>{formatTimestamp(event.timestamp)}</span>
                <span>
                  {event.event_type}
                  {event.tool_name && <span className="tool-count-badge">{event.tool_name}</span>}
                  {event.event_type === "tool_result" && event.tool_success !== undefined && event.tool_success !== null && (
                    <span className={event.tool_success ? "tool-result-badge--success" : "tool-result-badge--failed"}>
                      {event.tool_success ? "● Success" : "● Failed"}
                    </span>
                  )}
                </span>
              </div>
              
              {isExpanded && fullContent ? (
                <div className="tool-output-block">{fullContent}</div>
              ) : (
                <>
                  {event.content_preview && <p>{event.content_preview}</p>}
                  {!event.content_preview && event.tool_input_summary && <p>{event.tool_input_summary}</p>}
                </>
              )}

              {isTruncated && (
                <button
                  className="expand-toggle"
                  onClick={() => toggleExpand(event.event_id)}
                >
                  {isExpanded ? "Collapse" : "Expand"}
                </button>
              )}

              {event.tool_error && (
                <p className="error-text">{event.tool_error}</p>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
