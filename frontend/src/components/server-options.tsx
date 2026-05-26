"use client";

import { useState } from "react";
import type { ServerView, SetupOption } from "@/lib/servers";

// ServerOptions is the per-server hardening panel shown under a selected,
// ready server. Toggling changes pending state only; nothing happens to the
// box until the operator clicks confirm, which hands the desired set to
// onApply (the parent then streams the live apply over SSH).
export function ServerOptions({
  server,
  catalog,
  onApply,
}: {
  server: ServerView;
  catalog: SetupOption[];
  onApply: (desired: string[]) => void;
}) {
  const current = new Set(server.options ?? []);
  const [pending, setPending] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(catalog.map((o) => [o.id, current.has(o.id)])),
  );

  const desired = catalog.map((o) => o.id).filter((id) => pending[id]);
  const dirty = desired.length !== current.size || desired.some((id) => !current.has(id));

  return (
    <div className="border-t border-line px-4 py-4">
      <p className="text-[12px] text-muted">
        hardening settings — tick to add, untick to remove. changes are applied over SSH (as
        mountabo, with sudo) only after you confirm.
      </p>
      <ul className="mt-3 space-y-2">
        {catalog.map((o) => (
          <li key={o.id} className="rounded-lg border border-line bg-surface p-3">
            <label className="flex cursor-pointer gap-3">
              <input
                type="checkbox"
                checked={!!pending[o.id]}
                onChange={(e) => setPending((s) => ({ ...s, [o.id]: e.target.checked }))}
                className="mt-0.5 h-4 w-4 accent-lime"
              />
              <span>
                <span className="flex items-center gap-2 text-[13px] font-medium text-cream">
                  {o.name}
                  {current.has(o.id) && <span className="text-[10px] text-blue">applied</span>}
                </span>
                <span className="mt-1 block text-[12px] leading-5 text-muted">{o.description}</span>
              </span>
            </label>
          </li>
        ))}
        {catalog.length === 0 && <li className="text-[12px] text-muted">no options available.</li>}
      </ul>
      <button
        onClick={() => onApply(desired)}
        disabled={!dirty}
        className="mt-3 flex w-full items-center justify-center gap-2 rounded-lg border border-lime/50 bg-lime/[0.06] px-4 py-2.5 text-[12.5px] font-medium text-lime transition-colors hover:bg-lime/[0.12] disabled:cursor-not-allowed disabled:opacity-40"
      >
        {dirty ? "confirm & apply changes" : "no changes to apply"}
      </button>
    </div>
  );
}
