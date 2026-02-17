import type { RunPhase } from "../state/run-store";
import { useProjectGraphStore } from "../state/project-graph-store";
import { isLiveRun } from "../lib/run-phase";

type Props = {
  runId: string | null;
  phase: RunPhase;
  onCancel: () => void;
  projectName?: string;
};

function readablePhase(phase: RunPhase): string {
  switch (phase) {
    case "idle":
      return "Idle";
    case "starting":
      return "Starting";
    case "running":
      return "Running";
    case "cancelling":
      return "Cancelling";
    case "completed":
      return "Completed";
    case "failed":
      return "Failed";
    case "cancelled":
      return "Cancelled";
    default:
      return phase;
  }
}

export function RunToolbar({ runId, phase, onCancel, projectName }: Props) {
  const totalRuns = useProjectGraphStore((s) => s.runs.length);
  const canCancel = runId !== null && (phase === "running" || phase === "starting");
  const live = isLiveRun(phase);

  return (
    <section className="panel run-toolbar">
      <div className="run-meta">
        {live ? (
          <>
            <h2>Run</h2>
            <p>
              <span className="muted-text">ID:</span> {runId ?? "-"}
            </p>
            <p>
              <span className="muted-text">Status:</span> {readablePhase(phase)}
            </p>
          </>
        ) : (
          <>
            <h2>{projectName ?? "Project"}</h2>
            <p className="muted-text">
              {totalRuns} run{totalRuns !== 1 ? "s" : ""}
            </p>
            {phase !== "idle" && (
              <p>
                <span className="muted-text">Last run:</span> {readablePhase(phase)}
              </p>
            )}
          </>
        )}
      </div>
      {canCancel && (
        <button type="button" onClick={onCancel} className="danger-button">
          Cancel Run
        </button>
      )}
    </section>
  );
}
