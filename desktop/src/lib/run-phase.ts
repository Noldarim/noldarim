import type { RunPhase } from "../state/run-store";

export function isLiveRun(phase: RunPhase): boolean {
  return phase === "starting" || phase === "running" || phase === "cancelling";
}
