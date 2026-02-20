// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useCallback, useEffect, useRef } from "react";

import {
  getPipelineRun,
  getPipelineRunActivity
} from "../lib/api";
import type { AIActivityRecord, WsEnvelope } from "../lib/types";
import { connectPipelineStream, type WsConnection } from "../lib/ws";
import { useRunStore } from "../state/run-store";
import { incrementReconnectCount, recordHydrateLatency } from "../lib/debug";
import { messageFromError } from "../lib/formatting";

export function useRunConnection(serverUrl: string) {
  const wsRef = useRef<WsConnection | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const hydrateTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const currentRunIdRef = useRef<string | null>(null);

  // Keep ref in sync with store's runId for stale-fetch guards
  const runId = useRunStore((s) => s.runId);
  const phase = useRunStore((s) => s.phase);
  currentRunIdRef.current = runId;

  const {
    wsActivityReceived,
    snapshotApplied,
    reportError
  } = useRunStore.getState();

  const closeRealtime = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
    if (hydrateTimerRef.current) {
      clearTimeout(hydrateTimerRef.current);
      hydrateTimerRef.current = null;
    }
  }, []);

  const hydrateRun = useCallback(
    async (hRunId: string) => {
      const start = performance.now();
      const [hRun, activityBatch] = await Promise.all([
        getPipelineRun(serverUrl, hRunId),
        getPipelineRunActivity(serverUrl, hRunId)
      ]);
      recordHydrateLatency(Math.round(performance.now() - start));
      snapshotApplied(hRun, activityBatch.Activities ?? []);
    },
    [serverUrl, snapshotApplied]
  );

  const scheduleHydrate = useCallback(
    (sRunId: string) => {
      if (hydrateTimerRef.current) {
        return;
      }
      hydrateTimerRef.current = setTimeout(async () => {
        hydrateTimerRef.current = null;
        try {
          await hydrateRun(sRunId);
        } catch (err) {
          console.warn("[scheduleHydrate] hydration failed, will retry via poll:", err);
        }
      }, 250);
    },
    [hydrateRun]
  );

  const startRealtime = useCallback(
    (projectId: string, sRunId: string) => {
      closeRealtime();

      wsRef.current = connectPipelineStream(
        serverUrl,
        projectId,
        sRunId,
        (message: WsEnvelope) => {
          if (message.type === "error") {
            reportError(message.message ?? "WebSocket error");
            return;
          }

          if (message.event_type === "*models.AIActivityRecord" && message.payload) {
            const p = message.payload as Record<string, unknown>;
            if (typeof p.event_id === "string" && typeof p.event_type === "string") {
              wsActivityReceived(p as unknown as AIActivityRecord);
            }
          }

          scheduleHydrate(sRunId);
        },
        (errorMessage) => {
          incrementReconnectCount();
          reportError(errorMessage);
        }
      );

      // Background reconciliation poll
      pollRef.current = setInterval(() => {
        void hydrateRun(sRunId).catch((err) => {
          reportError(messageFromError(err));
        });
      }, 10_000);
    },
    [closeRealtime, hydrateRun, scheduleHydrate, serverUrl, reportError, wsActivityReceived]
  );

  // Post-completion tail hydrations
  useEffect(() => {
    if ((phase === "completed" || phase === "failed") && runId) {
      const capturedRunId = runId;
      const timers = [2_000, 5_000].map((delay) =>
        setTimeout(async () => {
          if (currentRunIdRef.current !== capturedRunId) {
            return;
          }
          try {
            const [tailRun, activityBatch] = await Promise.all([
              getPipelineRun(serverUrl, capturedRunId),
              getPipelineRunActivity(serverUrl, capturedRunId)
            ]);
            if (currentRunIdRef.current !== capturedRunId) {
              return;
            }
            snapshotApplied(tailRun, activityBatch.Activities ?? []);
          } catch {
            // Ignore errors during tail hydrations — run is already done.
          }
        }, delay)
      );

      return () => {
        for (const timer of timers) {
          clearTimeout(timer);
        }
      };
    }
  }, [phase, runId, snapshotApplied, serverUrl]);

  // Close WS + polling when the run reaches a terminal phase
  useEffect(() => {
    if (phase === "completed" || phase === "failed") {
      closeRealtime();
    }
  }, [phase, closeRealtime]);

  // Cleanup on unmount
  useEffect(() => () => { closeRealtime(); }, [closeRealtime]);

  return { startRealtime, closeRealtime, hydrateRun };
}
