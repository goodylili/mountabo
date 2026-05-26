"use client";

import { useState } from "react";
import { ChevronDown } from "@/components/icons";
import type { ServerView } from "@/lib/servers";

// ServerSelect is the deploy-target picker in the deploy bar. It replaces a
// native <select> (whose popup the browser positions and styles itself, so it
// never matched the app) with a dropdown that mirrors OwnerDropdown: an
// app-styled trigger and a panel anchored directly under it.
export function ServerSelect({
  servers,
  value,
  onChange,
}: {
  servers: ServerView[];
  value: string | null;
  onChange: (id: string | null) => void;
}) {
  const [open, setOpen] = useState(false);
  const selected = servers.find((s) => s.id === value) ?? null;
  const empty = servers.length === 0;

  return (
    <div className="relative">
      <button
        onClick={() => !empty && setOpen((o) => !o)}
        disabled={empty}
        className="flex w-full min-w-[15rem] items-center justify-between gap-2 rounded-md border border-line bg-surface px-3 py-2 text-[13px] text-cream transition-colors hover:border-line-strong focus:border-line-strong focus:outline-none disabled:opacity-60"
      >
        <span className={selected ? "text-cream" : "text-muted"}>
          {selected
            ? `${selected.name} · ${selected.ip}`
            : empty
              ? "no ready servers, set one up first"
              : "select a server…"}
        </span>
        <ChevronDown className={`text-muted transition-transform ${open ? "rotate-180" : ""}`} />
      </button>

      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
          <div className="absolute left-0 right-0 top-full z-20 mt-1 max-h-72 overflow-y-auto rounded-lg border border-line bg-surface py-1 shadow-xl">
            {servers.map((s) => (
              <button
                key={s.id}
                onClick={() => {
                  onChange(s.id);
                  setOpen(false);
                }}
                className={`block w-full px-3 py-2 text-left text-[13px] transition-colors ${
                  s.id === value ? "bg-lime/[0.1] text-cream" : "text-muted hover:bg-surface-hover hover:text-cream"
                }`}
              >
                {s.name} · {s.ip}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
