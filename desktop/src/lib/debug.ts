type DebugCounters = {
  droppedInvalidWs: number;
  reconnectCount: number;
  hydrateLatencyMs: number[];
};

const counters: DebugCounters = {
  droppedInvalidWs: 0,
  reconnectCount: 0,
  hydrateLatencyMs: []
};

export function incrementDroppedInvalidWs() {
  counters.droppedInvalidWs += 1;
  console.debug("[debug] dropped invalid WS payload, total:", counters.droppedInvalidWs);
}

export function incrementReconnectCount() {
  counters.reconnectCount += 1;
  console.debug("[debug] WS reconnect, total:", counters.reconnectCount);
}

export function recordHydrateLatency(ms: number) {
  counters.hydrateLatencyMs.push(ms);
  console.debug("[debug] hydrate latency:", ms, "ms");
}

export function getDebugCounters(): Readonly<DebugCounters> {
  return { ...counters, hydrateLatencyMs: [...counters.hydrateLatencyMs] };
}

export function resetDebugCounters() {
  counters.droppedInvalidWs = 0;
  counters.reconnectCount = 0;
  counters.hydrateLatencyMs = [];
}

// Expose in devtools
if (typeof window !== "undefined") {
  (window as unknown as Record<string, unknown>).__noldarimDebug = {
    getCounters: getDebugCounters,
    resetCounters: resetDebugCounters
  };
}
