type Props = {
  runId: string;
  phase: string;
  onCancel: () => void;
};

function readablePhase(phase: string): string {
  switch (phase) {
    case "starting":
      return "Starting";
    case "running":
      return "Running";
    case "cancelling":
      return "Cancelling";
    default:
      return phase;
  }
}

export function FloatingRunStatus({ runId, phase, onCancel }: Props) {
  const canCancel = phase === "running" || phase === "starting";

  return (
    <div className="floating-run-status">
      <span className="floating-run-status__phase">{readablePhase(phase)}</span>
      <span className="floating-run-status__id muted-text">{runId.slice(0, 8)}</span>
      {canCancel && (
        <button type="button" onClick={onCancel} className="danger-button">
          Cancel
        </button>
      )}
    </div>
  );
}
