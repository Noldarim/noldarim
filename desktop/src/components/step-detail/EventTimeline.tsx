// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import type { AIActivityRecord } from "../../lib/types";
import { formatTimestamp } from "../../lib/formatting";

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

  return (
    <section>
      <h4>Event timeline</h4>
      {events.length === 0 && <p className="muted-text">No events for this step yet.</p>}
      <ul className="event-list">
        {sortedEvents.map((event) => (
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
  );
}
