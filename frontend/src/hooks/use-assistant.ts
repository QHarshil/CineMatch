"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { AssistantRequestError, streamAssistant } from "@/lib/assistant-stream";
import { applyEvent, newExchange, toTurns, type Exchange } from "@/lib/assistant-session";
import { API_BASE, fetchAssistantUsage, type AssistantUsage } from "@/lib/api";

/** Owns the conversation, the live stream, and today's quota. */
export function useAssistant(token: string | undefined) {
  const [exchanges, setExchanges] = useState<Exchange[]>([]);
  const [usage, setUsage] = useState<AssistantUsage | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  // State updates land on the next render, so a second click in the same
  // frame would still see the old history. The ref blocks it synchronously.
  const sendingRef = useRef(false);
  const historyRef = useRef<Exchange[]>([]);
  historyRef.current = exchanges;

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    fetchAssistantUsage(token)
      .then((u) => {
        if (!cancelled) setUsage(u);
      })
      .catch(() => {
        // Quota display is optional; the stream reports limits too.
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  useEffect(() => () => abortRef.current?.abort(), []);

  const busy = exchanges.at(-1)?.status === "streaming";

  const send = useCallback(
    async (prompt: string) => {
      const text = prompt.trim();
      if (!token || !text || sendingRef.current) return;
      sendingRef.current = true;

      const id = crypto.randomUUID();
      const turns = toTurns(historyRef.current, text);
      const update = (fn: (ex: Exchange) => Exchange) =>
        setExchanges((all) => all.map((ex) => (ex.id === id ? fn(ex) : ex)));
      setExchanges((all) => [...all, newExchange(id, text)]);

      const controller = new AbortController();
      abortRef.current = controller;
      try {
        for await (const event of streamAssistant({ apiBase: API_BASE, token, turns, signal: controller.signal })) {
          update((ex) => applyEvent(ex, event));
          if (event.type === "done") {
            setUsage((u) => (u ? { ...u, remaining: event.data.remaining_today, used: u.used + 1 } : u));
          }
        }
        update((ex) => (ex.status === "streaming" ? { ...ex, status: "failed", error: { message: "The response ended early. Try again." } } : ex));
      } catch (err) {
        const aborted = controller.signal.aborted;
        const message = aborted
          ? "Stopped."
          : err instanceof AssistantRequestError
            ? err.message
            : "Could not reach the assistant. Check your connection and try again.";
        update((ex) => ({
          ...ex,
          status: "failed",
          error: {
            message,
            status: err instanceof AssistantRequestError ? err.status : undefined,
            resetsAt: err instanceof AssistantRequestError ? err.resetsAt : undefined,
          },
        }));
      } finally {
        sendingRef.current = false;
        if (abortRef.current === controller) abortRef.current = null;
      }
    },
    [token],
  );

  const stop = useCallback(() => abortRef.current?.abort(), []);
  const reset = useCallback(() => {
    abortRef.current?.abort();
    setExchanges([]);
  }, []);

  return { exchanges, usage, busy, send, stop, reset };
}
