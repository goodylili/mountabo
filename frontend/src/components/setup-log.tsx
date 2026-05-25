"use client";

import { useEffect, useRef, useState } from "react";
import type { ServerStatus, ServerView } from "@/lib/servers";

type Line = { at: Date; text: string; kind: "log" | "done" | "error" };

// SetupLog opens an EventSource to the SSE proxy, which triggers the backend to
// run the bootstrap and streams progress back. Each line is timestamped in the
// viewer's local (browser) timezone. The connection is closed on the terminal
// done/error event so EventSource does not auto-reconnect and re-run setup.
export function SetupLog({
  server,
  onStatus,
  onClose,
}: {
  server: ServerView;
  onStatus: (status: ServerStatus) => void;
  onClose: () => void;
}) {
  const [lines, setLines] = useState<Line[]>([]);
  const [finished, setFinished] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  // Keep onStatus in a ref so the effect depends ONLY on server.id. Otherwise a
  // fresh onStatus identity each render (and the state update it triggers) would
  // re-run the effect, reopen the EventSource, and re-run setup in a loop.
  const onStatusRef = useRef(onStatus);
  useEffect(() => {
    onStatusRef.current = onStatus;
  }, [onStatus]);

  useEffect(() => {
    const es = new EventSource(`/api/servers/${server.id}/setup`);

    const add = (text: string, kind: Line["kind"]) =>
      setLines((prev) => [...prev, { at: new Date(), text, kind }]);

    es.onmessage = (e) => add(e.data, "log");

    es.addEventListener("done", (e) => {
      add((e as MessageEvent).data || "server is ready", "done");
      setFinished(true);
      onStatusRef.current("ready");
      es.close(); // prevent EventSource auto-reconnect (which would re-run setup)
    });

    es.addEventListener("error", (e) => {
      const data = (e as MessageEvent).data;
      if (data) {
        // Backend-reported setup failure (a named SSE error event).
        add(data, "error");
        setFinished(true);
        onStatusRef.current("failed");
      }
      // On any error event, close so EventSource never silently reconnects and
      // re-triggers the bootstrap against the server.
      es.close();
    });

    return () => es.close();
  }, [server.id]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [lines]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-6">
      <div className="flex max-h-[80vh] w-full max-w-3xl flex-col overflow-hidden rounded-xl border border-line bg-surface">
        <div className="flex items-center justify-between border-b border-line px-5 py-3">
          <span className="flex items-center gap-2 text-[13px] text-cream">
            setting up <span className="font-medium">{server.name}</span>
            <span className="text-faint">·</span>
            <span className="text-muted">{server.ip}</span>
          </span>
          <button
            onClick={onClose}
            className="rounded-md border border-line px-2.5 py-1 text-[12px] text-muted transition-colors hover:text-cream"
          >
            {finished ? "close" : "hide"}
          </button>
        </div>

        <div ref={scrollRef} className="flex-1 overflow-y-auto bg-black/30 px-5 py-4 font-mono text-[12px] leading-6">
          {lines.length === 0 && <p className="text-muted">connecting to {server.ip}…</p>}
          {lines.map((l, i) => (
            <div key={i} className="flex gap-3">
              <span className="shrink-0 text-faint">{l.at.toLocaleTimeString()}</span>
              <span
                className={
                  l.kind === "error" ? "text-red-300" : l.kind === "done" ? "text-lime" : "text-body"
                }
              >
                {l.kind === "done" ? "✓ " : l.kind === "error" ? "✗ " : ""}
                {l.text}
              </span>
            </div>
          ))}
        </div>

        <div className="flex items-center justify-between border-t border-line px-5 py-2.5 text-[11px] text-muted">
          <span>timestamps in your timezone · {server.timezone}</span>
          <span>{finished ? "finished" : "streaming…"}</span>
        </div>
      </div>
    </div>
  );
}
