// usePullProgress — React hook for streaming ollama pull progress via SSE.
// Uses fetch + ReadableStream instead of EventSource to prevent auto-reconnect
// (EventSource would re-trigger ollama pull on reconnect — not desired).
// SPEC: SPEC-MINK-ONBOARDING-001 §6.3 Phase 3C
import { useState, useCallback, useEffect, useRef } from "react";
import type { PullProgressUpdate } from "@/types/onboarding";

// @MX:ANCHOR: [AUTO] SSE streaming hook — consumed by Step2Model; controls AbortController lifecycle.
// @MX:REASON: Cleanup on unmount must abort fetch to prevent memory leaks and ghost requests.
export interface UsePullProgressResult {
  isStreaming: boolean;
  latest: PullProgressUpdate | null;
  history: PullProgressUpdate[];
  error: string | null;
  done: boolean;
  start: (sessionId: string, modelName: string) => void;
  cancel: () => void;
}

// Parse raw SSE text into (event, data) pairs.
// SSE blocks are separated by \n\n; each block may have multiple lines.
// Concatenates multiple data: lines per SSE spec.
function parseSseBlock(block: string): { event: string; data: string } | null {
  const lines = block.split("\n");
  let event = "message";
  const dataParts: string[] = [];

  for (const line of lines) {
    if (line.startsWith("event:")) {
      event = line.slice(6).trim();
    } else if (line.startsWith("data:")) {
      dataParts.push(line.slice(5).trim());
    }
  }

  if (dataParts.length === 0) return null;
  return { event, data: dataParts.join("\n") };
}

// @MX:WARN: [AUTO] Async streaming loop with manual AbortController — mishandling causes dangling connections.
// @MX:REASON: fetch body reader must be released on abort; setState must not be called after unmount.
export function usePullProgress(): UsePullProgressResult {
  const [isStreaming, setIsStreaming] = useState(false);
  const [latest, setLatest] = useState<PullProgressUpdate | null>(null);
  const [history, setHistory] = useState<PullProgressUpdate[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  // AbortController ref keeps stable reference across renders.
  const abortRef = useRef<AbortController | null>(null);
  // Mounted flag prevents setState after unmount.
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      // Cancel any in-flight stream on unmount.
      abortRef.current?.abort();
    };
  }, []);

  const cancel = useCallback(() => {
    abortRef.current?.abort();
    if (mountedRef.current) {
      setIsStreaming(false);
    }
  }, []);

  const start = useCallback((sessionId: string, modelName: string) => {
    // Abort any previous stream before starting a new one.
    abortRef.current?.abort();

    const controller = new AbortController();
    abortRef.current = controller;

    if (!mountedRef.current) return;

    // Reset state for new pull.
    setIsStreaming(true);
    setLatest(null);
    setHistory([]);
    setError(null);
    setDone(false);

    const url =
      `/install/api/session/${encodeURIComponent(sessionId)}/pull/stream` +
      `?model=${encodeURIComponent(modelName)}`;

    void (async () => {
      let reader: ReadableStreamDefaultReader<Uint8Array> | null = null;
      try {
        const response = await fetch(url, {
          credentials: "include",
          signal: controller.signal,
        });

        if (!response.ok) {
          if (mountedRef.current) {
            setError(`Failed to start pull stream: HTTP ${response.status}`);
            setIsStreaming(false);
          }
          return;
        }

        if (!response.body) {
          if (mountedRef.current) {
            setError("Response body is empty.");
            setIsStreaming(false);
          }
          return;
        }

        reader = response.body.getReader();
        const decoder = new TextDecoder();
        // Buffer to accumulate incomplete SSE chunks between reads.
        let buffer = "";

        while (true) {
          const { done: streamDone, value } = await reader.read();
          if (streamDone) break;

          buffer += decoder.decode(value, { stream: true });

          // Process all complete SSE blocks (separated by \n\n).
          const blocks = buffer.split("\n\n");
          // The last element may be an incomplete block — keep it in buffer.
          buffer = blocks.pop() ?? "";

          for (const block of blocks) {
            const trimmed = block.trim();
            if (!trimmed) continue;

            const parsed = parseSseBlock(trimmed);
            if (!parsed) continue;

            const { event, data } = parsed;

            if (event === "done") {
              if (mountedRef.current) {
                setDone(true);
                setIsStreaming(false);
              }
              return;
            }

            if (event === "error") {
              if (mountedRef.current) {
                try {
                  const errPayload = JSON.parse(data) as { message: string };
                  setError(errPayload.message);
                } catch {
                  setError(data);
                }
                setIsStreaming(false);
              }
              return;
            }

            // Default "message" event — parse as PullProgressUpdate.
            if (event === "message") {
              try {
                const update = JSON.parse(data) as PullProgressUpdate;
                if (mountedRef.current) {
                  setLatest(update);
                  setHistory((prev) => [...prev, update]);
                }
              } catch {
                // Ignore malformed JSON frames silently.
              }
            }
          }
        }
      } catch (err) {
        // Ignore AbortError — intentional cancel.
        if (err instanceof Error && err.name === "AbortError") return;
        if (mountedRef.current) {
          setError(err instanceof Error ? err.message : "Stream error");
          setIsStreaming(false);
        }
      } finally {
        try {
          await reader?.cancel();
        } catch {
          // Best-effort release.
        }
        if (mountedRef.current && !controller.signal.aborted) {
          setIsStreaming(false);
        }
      }
    })();
  }, []);

  return { isStreaming, latest, history, error, done, start, cancel };
}
