"use client";

import { useEffect, useRef, useState } from "react";

type Line = { at: Date; text: string; kind: "log" | "done" | "error" };

// StreamLog shows a live Server-Sent Events log with local-timezone timestamps.
// With no `body` it opens an EventSource GET to `url` (setup, apply-options);
// with a `body` it POSTs the JSON body to `url` and parses the streamed SSE
// response (deploy, whose body carries secrets and so cannot be a GET). Either
// way it stops on the terminal done/error event so the operation never re-runs,
// and onDone(ok) fires once when finished.
export function StreamLog({
  title,
  subtitle,
  timezone,
  url,
  body,
  onClose,
  onDone,
}: {
  title: string;
  subtitle?: string;
  timezone?: string;
  url: string;
  body?: unknown;
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

  // Serialize the body so a stable value drives the effect (an inline object
  // would re-run it and re-POST). undefined => GET/EventSource mode.
  const bodyJSON = body === undefined ? undefined : JSON.stringify(body);

  useEffect(() => {
    const add = (text: string, kind: Line["kind"]) =>
      setLines((prev) => [...prev, { at: new Date(), text, kind }]);
    const finish = (ok: boolean) => {
      setFinished(true);
      onDoneRef.current?.(ok);
    };

    // GET / EventSource mode.
    if (bodyJSON === undefined) {
      const es = new EventSource(url);
      es.onmessage = (e) => add(e.data, "log");
      es.addEventListener("done", (e) => {
        add((e as MessageEvent).data || "done", "done");
        finish(true);
        es.close();
      });
      es.addEventListener("error", (e) => {
        const data = (e as MessageEvent).data;
        if (data) {
          add(data, "error");
          finish(false);
        }
        es.close();
      });
      return () => es.close();
    }

    // POST + streamed-SSE-response mode.
    const ctrl = new AbortController();
    (async () => {
      let resp: Response;
      try {
        resp = await fetch(url, {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: bodyJSON,
          signal: ctrl.signal,
        });
      } catch {
        if (!ctrl.signal.aborted) {
          add("could not reach the server", "error");
          finish(false);
        }
        return;
      }
      if (!resp.ok || !resp.body) {
        add("request failed", "error");
        finish(false);
        return;
      }

      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      for (;;) {
        let chunk: ReadableStreamReadResult<Uint8Array>;
        try {
          chunk = await reader.read();
        } catch {
          break; // aborted on unmount
        }
        if (chunk.done) break;
        buf += decoder.decode(chunk.value, { stream: true });

        // SSE frames are separated by a blank line.
        let sep: number;
        while ((sep = buf.indexOf("\n\n")) >= 0) {
          const frame = buf.slice(0, sep);
          buf = buf.slice(sep + 2);
          let event = "message";
          let data = "";
          for (const raw of frame.split("\n")) {
            if (raw.startsWith("event:")) event = raw.slice(6).trim();
            else if (raw.startsWith("data:")) data = raw.slice(5).replace(/^ /, "");
          }
          if (event === "done") {
            add(data || "done", "done");
            finish(true);
          } else if (event === "error") {
            if (data) add(data, "error");
            finish(false);
          } else {
            add(data, "log");
          }
        }
      }
    })();
    return () => ctrl.abort();
  }, [url, bodyJSON]);

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
