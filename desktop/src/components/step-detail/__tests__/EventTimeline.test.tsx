import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { EventTimeline } from "../EventTimeline";
import type { AIActivityRecord } from "../../../lib/types";

function makeEvent(overrides?: Partial<AIActivityRecord>): AIActivityRecord {
  return {
    event_id: "evt-1",
    task_id: "t1",
    run_id: "r1",
    event_type: "tool_use",
    timestamp: "2026-02-14T10:10:00Z",
    ...overrides
  };
}

describe("EventTimeline", () => {
  it("renders empty state when no events", () => {
    render(<EventTimeline events={[]} />);
    expect(screen.getByText("No events for this step yet.")).toBeInTheDocument();
  });

  it("renders event type and content preview", () => {
    render(<EventTimeline events={[makeEvent({ content_preview: "Hello world" })]} />);
    expect(screen.getByText("tool_use")).toBeInTheDocument();
    expect(screen.getByText("Hello world")).toBeInTheDocument();
  });

  it("falls back to tool_input_summary when no content_preview", () => {
    render(<EventTimeline events={[makeEvent({ tool_input_summary: "Read main.go" })]} />);
    expect(screen.getByText("Read main.go")).toBeInTheDocument();
  });

  it("renders multiple events", () => {
    const events = [
      makeEvent({ event_id: "e1", event_type: "tool_use", content_preview: "first" }),
      makeEvent({ event_id: "e2", event_type: "tool_result", content_preview: "second" })
    ];
    render(<EventTimeline events={events} />);
    expect(screen.getByText("first")).toBeInTheDocument();
    expect(screen.getByText("second")).toBeInTheDocument();
  });

  it("renders events in chronological order", () => {
    const events = [
      makeEvent({ event_id: "e2", timestamp: "2026-02-14T10:10:02Z", content_preview: "second" }),
      makeEvent({ event_id: "e1", timestamp: "2026-02-14T10:10:01Z", content_preview: "first" })
    ];
    render(<EventTimeline events={events} />);

    const items = screen.getAllByRole("listitem");
    expect(items[0]).toHaveTextContent("first");
    expect(items[1]).toHaveTextContent("second");
  });
});
