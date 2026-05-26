"use client";

import { useEffect, useRef, useState } from "react";

type Line = { at: Date; text: string; kind: "log" | "done" | "error" };

// StreamLog opens an EventSource to `url` (which triggers the backend op) and
// shows the live log with local-timezone timestamps. It closes the connection
// on the terminal done/error event so EventSource never auto-reconnects and
// re-runs the operation. onDone(ok) fires once when finished.
export function StreamLog({
  title,
  subtitle,
  timezone,
  url,
  onClose,
  onDone,
}: {
  title: string;
  subtitle?: string;
  timezone?: string;
  url: string;
  onClose: () => void;
  onDone?: (ok: boolean) => void;
}) {
  const [lines, setLines] = useState<Line[]>([]);
  const [finished, setFinished] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  const onDoneRef = useRef(onDone);
  useEffect(() => {
    onDoneRef.current = onDone;
  }, [onDone]);

  useEffect(() => {
    const es = new EventSource(url);
    const add = (text: string, kind: Line["kind"]) =>
      setLines((prev) => [...prev, { at: new Date(), text, kind }]);

    es.onmessage = (e) => add(e.data, "log");
    es.addEventListener("done", (e) => {
      add((e as MessageEvent).data || "done", "done");
      setFinished(true);
      onDoneRef.current?.(true);
      es.close();
    });
    es.addEventListener("error", (e) => {
      const data = (e as MessageEvent).data;
      if (data) {
        add(data, "error");
        setFinished(true);
        onDoneRef.current?.(false);
      }
      es.close();
    });
    return () => es.close();
  }, [url]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [lines]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-3 sm:p-6">
      <div className="flex max-h-[85vh] w-full max-w-3xl flex-col overflow-hidden rounded-xl border border-line bg-surface sm:max-h-[80vh]">
        <div className="flex items-center justify-between gap-3 border-b border-line px-4 py-3 sm:px-5">
          <span className="flex min-w-0 items-center gap-2 text-[13px] text-cream">
            <span className="truncate">{title}</span>
            {subtitle && (
              <>
                <span className="text-faint">·</span>
                <span className="truncate text-muted">{subtitle}</span>
              </>
            )}
          </span>
          <button
            onClick={onClose}
            className="shrink-0 rounded-md border border-line px-2.5 py-1 text-[12px] text-muted transition-colors hover:text-cream"
          >
            {finished ? "close" : "hide"}
          </button>
        </div>

        <div
          ref={scrollRef}
          className="flex-1 overflow-y-auto overscroll-contain break-words bg-black/30 px-4 py-4 font-mono text-[12px] leading-6 sm:px-5"
        >
          {lines.length === 0 && <p className="text-muted">connecting…</p>}
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

        <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 border-t border-line px-4 py-2.5 text-[11px] text-muted sm:px-5">
          <span>timestamps in your timezone{timezone ? ` · ${timezone}` : ""}</span>
          <span>{finished ? "finished" : "streaming…"}</span>
        </div>
      </div>
    </div>
  );
}
