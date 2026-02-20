export function durationToMs(duration?: number): number {
  if (!duration || duration <= 0) return 0;

  // Backend step durations are always stored as nanoseconds.
  return duration / 1_000_000;
}

export function formatDuration(durationMs?: number): string {
  if (!durationMs || durationMs <= 0) return "0s";
  if (durationMs < 1_000) return `${Math.round(durationMs)}ms`;
  return `${(durationMs / 1_000).toFixed(1)}s`;
}
