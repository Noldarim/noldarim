export type GraphSelection =
  | { kind: "run-edge"; runId: string }
  | { kind: "step-edge"; runId: string; stepId: string };
