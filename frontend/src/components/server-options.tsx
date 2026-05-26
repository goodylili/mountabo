"use client";

import { useState } from "react";
import type { ServerView, SetupOption } from "@/lib/servers";

const CATEGORY_ORDER = ["Network", "SSH", "Monitoring", "System", "Audit"];

// ServerOptions is the per-server hardening panel shown under a selected, ready
// server. Options are grouped into category sub-pages (Network, SSH, …). Toggling
// changes pending state only; nothing happens to the box until the operator
// clicks confirm, which hands the desired set to onApply (the parent streams the
// live apply over SSH).
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
  const [pending, setPending] = useState<Record<string, boolean>>({});
  const [activeCat, setActiveCat] = useState<string | null>(null);

  // Whether an option is on in the pending view (falls back to its applied state
  // for options the user hasn't touched — robust if the catalog loads late).
  const isOn = (id: string) => pending[id] ?? current.has(id);

  const categories = CATEGORY_ORDER.filter((c) => catalog.some((o) => o.category === c));
  for (const o of catalog) if (!categories.includes(o.category)) categories.push(o.category);
  const active = activeCat && categories.includes(activeCat) ? activeCat : categories[0] ?? "";
  const items = catalog.filter((o) => o.category === active);

  const desired = catalog.map((o) => o.id).filter(isOn);
  const changed = catalog.filter((o) => isOn(o.id) !== current.has(o.id)).length;

  return (
    <div className="border-t border-line px-4 py-4">
      <p className="text-[12px] text-muted">
        hardening settings — tick to add, untick to remove. changes are applied over SSH (as
        mountabo, with sudo) only after you confirm.
      </p>

      {/* category sub-pages */}
      <div className="mt-3 flex flex-wrap gap-1">
        {categories.map((c) => {
          const onCount = catalog.filter((o) => o.category === c && isOn(o.id)).length;
          return (
            <button
              key={c}
              onClick={() => setActiveCat(c)}
              className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 text-[12px] transition-colors ${
                c === active ? "bg-surface text-cream" : "text-muted hover:text-cream"
              }`}
            >
              {c.toLowerCase()}
              {onCount > 0 && (
                <span className="rounded bg-lime/15 px-1.5 text-[10px] text-lime">{onCount}</span>
              )}
            </button>
          );
        })}
      </div>

      <ul className="mt-3 space-y-2">
        {items.map((o) => (
          <li key={o.id} className="rounded-lg border border-line bg-surface p-3">
            <label className="flex cursor-pointer gap-3">
              <input
                type="checkbox"
                checked={isOn(o.id)}
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
      </ul>

      <button
        onClick={() => onApply(desired)}
        disabled={changed === 0}
        className="mt-3 flex w-full items-center justify-center gap-2 rounded-lg border border-lime/50 bg-lime/[0.06] px-4 py-2.5 text-[12.5px] font-medium text-lime transition-colors hover:bg-lime/[0.12] disabled:cursor-not-allowed disabled:opacity-40"
      >
        {changed === 0 ? "no changes to apply" : `confirm & apply ${changed} change${changed === 1 ? "" : "s"}`}
      </button>
    </div>
  );
}
