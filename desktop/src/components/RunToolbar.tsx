import type { RunPhase } from "../state/run-store";

type Props = {
  runId: string | null;
  phase: RunPhase;
  onCancel: () => void;
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

export function RunToolbar({ runId, phase, onCancel }: Props) {
  const canCancel = runId !== null && (phase === "running" || phase === "starting");

  return (
    <section className="panel run-toolbar">
      <div className="run-meta">
        <h2>Run</h2>
        <p>
          <span className="muted-text">ID:</span> {runId ?? "-"}
        </p>
        <p>
          <span className="muted-text">Status:</span> {readablePhase(phase)}
        </p>
      </div>
      {canCancel && (
        <button type="button" onClick={onCancel} className="danger-button">
          Cancel Run
        </button>
      )}
    </section>
  );
}
