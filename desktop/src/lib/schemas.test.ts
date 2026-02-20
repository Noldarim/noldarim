// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { describe, expect, it } from "vitest";

import {
  AgentDefaultsSchema,
  AIActivityBatchEventSchema,
  AIActivityRecordSchema,
  CancelPipelineResultSchema,
  PipelineRunSchema,
  PipelineRunResultSchema,
  ProjectsLoadedEventSchema,
  WsEnvelopeSchema
} from "./schemas";

describe("ProjectsLoadedEventSchema", () => {
  it("parses valid projects payload", () => {
    const result = ProjectsLoadedEventSchema.parse({
      Projects: {
        "proj-1": { id: "proj-1", name: "My Project", description: "desc", repository_path: "/path" }
      }
    });
    expect(result.Projects["proj-1"].name).toBe("My Project");
  });

  it("coerces null Projects to empty object", () => {
    const result = ProjectsLoadedEventSchema.parse({ Projects: null });
    expect(result.Projects).toEqual({});
  });

  it("rejects missing Projects field", () => {
    expect(() => ProjectsLoadedEventSchema.parse({})).toThrow();
  });
});

describe("AgentDefaultsSchema", () => {
  it("parses valid defaults", () => {
    const result = AgentDefaultsSchema.parse({
      tool_name: "claude",
      tool_version: "1.0",
      flag_format: "space",
      tool_options: { key: "value" }
    });
    expect(result.tool_name).toBe("claude");
  });

  it("rejects missing tool_name", () => {
    expect(() =>
      AgentDefaultsSchema.parse({ tool_version: "1.0", flag_format: "space", tool_options: {} })
    ).toThrow();
  });
});

describe("PipelineRunSchema", () => {
  it("parses run with step_results", () => {
    const result = PipelineRunSchema.parse({
      id: "run-1",
      project_id: "proj-1",
      name: "Pipeline",
      status: 1,
      step_results: [
        {
          id: "sr-1",
          pipeline_run_id: "run-1",
          step_id: "step-1",
          step_index: 0,
          status: 2,
          commit_sha: "abc",
          commit_message: "msg",
          git_diff: "",
          files_changed: 1,
          insertions: 2,
          deletions: 3,
          input_tokens: 100,
          output_tokens: 50,
          cache_read_tokens: 0,
          cache_create_tokens: 0,
          agent_output: "",
          duration: 10
        }
      ]
    });
    expect(result.step_results).toHaveLength(1);
    expect(result.step_results![0].step_id).toBe("step-1");
  });

  it("parses run without step_results", () => {
    const result = PipelineRunSchema.parse({
      id: "run-1",
      project_id: "proj-1",
      name: "Pipeline",
      status: 0
    });
    expect(result.step_results).toBeUndefined();
  });

  it("rejects non-numeric status", () => {
    expect(() =>
      PipelineRunSchema.parse({ id: "run-1", project_id: "proj-1", name: "P", status: "running" })
    ).toThrow();
  });
});

describe("PipelineRunResultSchema", () => {
  it("parses start-pipeline response", () => {
    const result = PipelineRunResultSchema.parse({
      RunID: "run-1",
      ProjectID: "proj-1",
      Name: "Pipeline",
      WorkflowID: "wf-1"
    });
    expect(result.RunID).toBe("run-1");
  });
});

describe("AIActivityRecordSchema", () => {
  it("parses minimal activity record", () => {
    const result = AIActivityRecordSchema.parse({
      event_id: "evt-1",
      task_id: "task-1",
      run_id: "run-1",
      event_type: "tool_use",
      timestamp: "2026-02-14T10:00:00Z"
    });
    expect(result.event_id).toBe("evt-1");
  });

  it("rejects missing event_type", () => {
    expect(() =>
      AIActivityRecordSchema.parse({ event_id: "e", task_id: "t", run_id: "r", timestamp: "ts" })
    ).toThrow();
  });
});

describe("AIActivityBatchEventSchema", () => {
  it("parses batch with activities", () => {
    const result = AIActivityBatchEventSchema.parse({
      TaskID: "task-1",
      ProjectID: "proj-1",
      Activities: [
        { event_id: "e", task_id: "t", run_id: "r", event_type: "tool_use", timestamp: "ts" }
      ]
    });
    expect(result.Activities).toHaveLength(1);
  });

  it("coerces null Activities to empty array", () => {
    const result = AIActivityBatchEventSchema.parse({
      TaskID: "task-1",
      ProjectID: "proj-1",
      Activities: null
    });
    expect(result.Activities).toEqual([]);
  });
});

describe("WsEnvelopeSchema", () => {
  it("parses event envelope", () => {
    const result = WsEnvelopeSchema.parse({
      type: "event",
      event_type: "*models.AIActivityRecord",
      payload: { event_id: "e1" }
    });
    expect(result.type).toBe("event");
    expect(result.event_type).toBe("*models.AIActivityRecord");
  });

  it("parses error envelope", () => {
    const result = WsEnvelopeSchema.parse({
      type: "error",
      message: "something went wrong"
    });
    expect(result.type).toBe("error");
    expect(result.message).toBe("something went wrong");
  });

  it("rejects unknown type", () => {
    const result = WsEnvelopeSchema.safeParse({ type: "unknown" });
    expect(result.success).toBe(false);
  });

  it("rejects completely invalid payload", () => {
    const result = WsEnvelopeSchema.safeParse("not an object");
    expect(result.success).toBe(false);
  });

  it("rejects number input", () => {
    const result = WsEnvelopeSchema.safeParse(42);
    expect(result.success).toBe(false);
  });
});

describe("CancelPipelineResultSchema", () => {
  it("parses cancel result", () => {
    const result = CancelPipelineResultSchema.parse({
      RunID: "run-1",
      Reason: "User cancelled",
      WorkflowStatus: "terminated"
    });
    expect(result.RunID).toBe("run-1");
  });
});
