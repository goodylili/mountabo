"use client";

import { useEffect, useState } from "react";
import type { ServerView } from "@/lib/servers";

// The browser's IANA timezone (e.g. "Africa/Lagos"), used to prefill the field
// and as the server's timezone. Falls back to UTC if unavailable.
function browserTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

// Non-sensitive form fields are remembered between visits so a refresh doesn't
// lose them. The root password is NEVER stored here — it goes only to the
// backend, which keeps it in the OS keychain. (AddServerForm only mounts after a
// client click, so reading localStorage at init is safe — no SSR hydration.)
const DRAFT_KEY = "mountabo:add-server-draft";

type Draft = { name?: string; ip?: string; port?: string; timezone?: string };

function readDraft(): Draft {
  if (typeof window === "undefined") return {};
  try {
    return JSON.parse(window.localStorage.getItem(DRAFT_KEY) ?? "{}") as Draft;
  } catch {
    return {};
  }
}

export function AddServerForm({
  onAdded,
  onCancel,
}: {
  onAdded: (server: ServerView) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState(() => readDraft().name ?? "");
  const [ip, setIp] = useState(() => readDraft().ip ?? "");
  const [port, setPort] = useState(() => readDraft().port ?? "22");
  const [timezone, setTimezone] = useState(() => readDraft().timezone ?? browserTimezone());
  const [rootPassword, setRootPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Persist the non-sensitive fields (never the password) so they survive a
  // reload. Writing to localStorage here does not set React state, so it is safe
  // in an effect.
  useEffect(() => {
    try {
      window.localStorage.setItem(DRAFT_KEY, JSON.stringify({ name, ip, port, timezone }));
    } catch {
      // localStorage unavailable (private mode / disabled) — drafts just won't persist.
    }
  }, [name, ip, port, timezone]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const resp = await fetch("/api/servers", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ name, ip, port: Number(port) || 22, timezone, rootPassword }),
      });
      if (resp.ok) {
        try {
          window.localStorage.removeItem(DRAFT_KEY);
        } catch {
          // ignore
        }
        onAdded((await resp.json()) as ServerView);
        return;
      }
      const data = (await resp.json().catch(() => ({}))) as { error?: string };
      setError(data.error ?? "could not add server");
    } catch {
      setError("could not reach mountabo");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="rounded-lg border border-line bg-surface-2 p-4">
      <div className="mb-3 flex items-center justify-between">
        <span className="label">add a server</span>
        <button type="button" onClick={onCancel} className="text-[12px] text-muted hover:text-cream">
          cancel
        </button>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <Field label="name">
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="edge-1" className={inputCls} required />
        </Field>
        <Field label="ip address">
          <input value={ip} onChange={(e) => setIp(e.target.value)} placeholder="203.0.113.10" className={inputCls} required />
        </Field>
        <Field label="ssh port">
          <input value={port} onChange={(e) => setPort(e.target.value)} inputMode="numeric" className={inputCls} />
        </Field>
        <Field label="timezone (auto-detected)">
          <input value={timezone} onChange={(e) => setTimezone(e.target.value)} placeholder="Africa/Lagos" className={inputCls} required />
        </Field>
        <div className="col-span-2">
          <Field label="root password (used once over ssh, then discarded)">
            <input
              type="password"
              value={rootPassword}
              onChange={(e) => setRootPassword(e.target.value)}
              placeholder="from your provider"
              className={inputCls}
              required
            />
          </Field>
        </div>
      </div>

      {error && <p className="mt-3 text-[12px] text-red-300">{error}</p>}

      <button
        type="submit"
        disabled={busy}
        className="mt-4 flex w-full items-center justify-center gap-2 rounded-lg bg-lime-fill px-4 py-3 text-[13px] font-bold text-black transition-transform hover:-translate-y-0.5 disabled:opacity-60"
      >
        {busy ? "connecting & probing…" : "connect & probe specs"}
      </button>
    </form>
  );
}

const inputCls =
  "w-full rounded-lg border border-line bg-surface px-3 py-2 text-[13px] text-cream placeholder:text-muted focus:border-line-strong focus:outline-none";

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-[11px] text-muted">{label}</span>
      {children}
    </label>
  );
}
