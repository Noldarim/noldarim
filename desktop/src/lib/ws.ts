import type { WsEnvelope } from "./types";
import { WsEnvelopeSchema } from "./schemas";
import { incrementDroppedInvalidWs } from "./debug";

type EventHandler = (message: WsEnvelope) => void;

type ErrorHandler = (message: string) => void;

function toWebSocketUrl(baseUrl: string): string {
  const normalized = new URL(baseUrl);
  const protocol = normalized.protocol === "https:" ? "wss:" : "ws:";
  normalized.protocol = protocol;
  normalized.pathname = "/ws";
  normalized.search = "";
  normalized.hash = "";
  return normalized.toString();
}

export type WsConnection = {
  close: () => void;
};

const INITIAL_BACKOFF_MS = 1_000;
const MAX_BACKOFF_MS = 30_000;

export function connectPipelineStream(
  baseUrl: string,
  projectId: string,
  runId: string,
  onEvent: EventHandler,
  onError: ErrorHandler
): WsConnection {
  let disposed = false;
  let backoff = INITIAL_BACKOFF_MS;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let currentWs: WebSocket | null = null;

  function subscribe(ws: WebSocket) {
    ws.send(JSON.stringify({ type: "subscribe", filters: { project_id: projectId } }));
    ws.send(JSON.stringify({ type: "subscribe", filters: { run_id: runId } }));
  }

  function connect() {
    if (disposed) {
      return;
    }

    const ws = new WebSocket(toWebSocketUrl(baseUrl));
    currentWs = ws;

    ws.addEventListener("open", () => {
      backoff = INITIAL_BACKOFF_MS;
      subscribe(ws);
    });

    ws.addEventListener("message", (event) => {
      let raw: unknown;
      try {
        raw = JSON.parse(String(event.data));
      } catch {
        console.debug("[ws] Received non-JSON payload, dropping");
        incrementDroppedInvalidWs();

        return;
      }

      const result = WsEnvelopeSchema.safeParse(raw);
      if (!result.success) {
        console.debug("[ws] Invalid WS envelope, dropping:", result.error);
        incrementDroppedInvalidWs();

        return;
      }

      onEvent(result.data);
    });

    ws.addEventListener("error", () => {
      onError("WebSocket connection error");
    });

    ws.addEventListener("close", (event) => {
      if (disposed) {
        return;
      }
      if (event.code !== 1000 && event.code !== 1001) {
        onError("WebSocket disconnected unexpectedly");
        scheduleReconnect();
      }
    });
  }

  function scheduleReconnect() {
    if (disposed || reconnectTimer !== null) {
      return;
    }
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      backoff = Math.min(backoff * 2, MAX_BACKOFF_MS);
      connect();
    }, backoff);
  }

  connect();

  return {
    close: () => {
      disposed = true;
      if (reconnectTimer !== null) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      if (currentWs && (currentWs.readyState === WebSocket.OPEN || currentWs.readyState === WebSocket.CONNECTING)) {
        currentWs.close();
      }
      currentWs = null;
    }
  };
}
