"use client";

import { useEffect, useRef, useState } from "react";
import type { ServerStatus, ServerView, SetupOption } from "@/lib/servers";

type Line = { at: Date; text: string; kind: "log" | "done" | "error" };

// SetupLog runs in two phases:
//  1. choose — fetch the opt-in hardening catalog and let the operator tick the
//     ones they want (all OFF by default; each shows what it does + its trade-off).
//  2. running — open the SSE stream (with ?options=…), which triggers the
//     backend bootstrap, and show the live log with timestamps in the local tz.
// The EventSource is closed on any done/error so it never auto-reconnects and
// re-runs setup.
export function SetupLog({
  server,
  onStatus,
  onClose,
}: {
  server: ServerView;
  onStatus: (status: ServerStatus) => void;
  onClose: () => void;
}) {
  const [options, setOptions] = useState<SetupOption[]>([]);
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [setupUrl, setSetupUrl] = useState<string | null>(null);
  const [lines, setLines] = useState<Line[]>([]);
  const [finished, setFinished] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  const onStatusRef = useRef(onStatus);
  useEffect(() => {
    onStatusRef.current = onStatus;
  }, [onStatus]);

  useEffect(() => {
    let active = true;
    fetch("/api/servers/options")
      .then((r) => r.json())
      .then((o) => {
        if (active) setOptions(o as SetupOption[]);
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!setupUrl) return;
    const es = new EventSource(setupUrl);

    const add = (text: string, kind: Line["kind"]) =>
      setLines((prev) => [...prev, { at: new Date(), text, kind }]);

    es.onmessage = (e) => add(e.data, "log");

    es.addEventListener("done", (e) => {
      add((e as MessageEvent).data || "server is ready", "done");
      setFinished(true);
      onStatusRef.current("ready");
      es.close();
    });

    es.addEventListener("error", (e) => {
      const data = (e as MessageEvent).data;
      if (data) {
        add(data, "error");
        setFinished(true);
        onStatusRef.current("failed");
      }
      es.close();
    });

    return () => es.close();
  }, [setupUrl]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [lines]);

  function start() {
    const ids = options.map((o) => o.id).filter((id) => selected[id]);
    const qs = ids.length ? `?options=${encodeURIComponent(ids.join(","))}` : "";
    setSetupUrl(`/api/servers/${server.id}/setup${qs}`);
  }

  const choosing = setupUrl === null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-6">
      <div className="flex max-h-[85vh] w-full max-w-3xl flex-col overflow-hidden rounded-xl border border-line bg-surface">
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
            {choosing ? "cancel" : finished ? "close" : "hide"}
          </button>
        </div>

        {choosing ? (
          <div className="flex-1 overflow-y-auto px-5 py-4">
            <p className="text-[13px] leading-6 text-body">
              mountabo will create the <code className="text-cream">mountabo</code> user, install
              your SSH keys, and install Docker. It won&apos;t change anything else. Add any optional
              hardening below — all off by default, your choice:
            </p>
            <ul className="mt-4 space-y-2">
              {options.map((o) => (
                <li key={o.id} className="rounded-lg border border-line bg-surface-2 p-4">
                  <label className="flex cursor-pointer gap-3">
                    <input
                      type="checkbox"
                      checked={!!selected[o.id]}
                      onChange={(e) => setSelected((s) => ({ ...s, [o.id]: e.target.checked }))}
                      className="mt-0.5 h-4 w-4 accent-lime"
                    />
                    <span>
                      <span className="text-[14px] font-medium text-cream">{o.name}</span>
                      <span className="mt-1 block text-[12.5px] leading-6 text-muted">
                        {o.description}
                      </span>
                    </span>
                  </label>
                </li>
              ))}
              {options.length === 0 && (
                <li className="text-[12.5px] text-muted">no optional steps available.</li>
              )}
            </ul>
            <button
              onClick={start}
              className="mt-5 flex w-full items-center justify-center gap-2 rounded-lg bg-lime-fill px-4 py-3 text-[13px] font-bold text-black transition-transform hover:-translate-y-0.5"
            >
              start setup
            </button>
          </div>
        ) : (
          <>
            <div
              ref={scrollRef}
              className="flex-1 overflow-y-auto bg-black/30 px-5 py-4 font-mono text-[12px] leading-6"
            >
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
          </>
        )}
      </div>
    </div>
  );
}
