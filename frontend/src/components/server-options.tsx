"use client";

import { useState } from "react";
import type { OptionParam, ServerView, SetupOption } from "@/lib/servers";

const CATEGORY_ORDER = ["Network", "SSH", "TLS", "Monitoring", "System", "Audit"];

// ServerOptions is the per-server hardening panel shown under a selected, ready
// server. Options are grouped into category sub-pages. Toggling changes pending
// state only; options that need parameters reveal inline inputs the moment they
// are ticked. Nothing is applied until the operator confirms, which hands the
// desired set + collected params to onApply (the parent streams the live apply).
export function ServerOptions({
  server,
  catalog,
  onApply,
}: {
  server: ServerView;
  catalog: SetupOption[];
  onApply: (desired: string[], params: Record<string, Record<string, string>>) => void;
}) {
  const current = new Set(server.options ?? []);
  const [pending, setPending] = useState<Record<string, boolean>>({});
  const [paramValues, setParamValues] = useState<Record<string, Record<string, string>>>({});
  const [activeCat, setActiveCat] = useState<string | null>(null);

  const isOn = (id: string) => pending[id] ?? current.has(id);
  const paramVal = (id: string, p: OptionParam) => paramValues[id]?.[p.key] ?? p.default ?? "";
  const setParam = (id: string, key: string, val: string) =>
    setParamValues((s) => ({ ...s, [id]: { ...(s[id] ?? {}), [key]: val } }));

  const categories = CATEGORY_ORDER.filter((c) => catalog.some((o) => o.category === c));
  for (const o of catalog) if (!categories.includes(o.category)) categories.push(o.category);
  const active = activeCat && categories.includes(activeCat) ? activeCat : categories[0] ?? "";
  const items = catalog.filter((o) => o.category === active);

  const changed = catalog.filter((o) => isOn(o.id) !== current.has(o.id)).length;
  // A ticked option with required params left blank can't be applied.
  const missingParams = catalog.some(
    (o) => isOn(o.id) && (o.params ?? []).some((p) => !paramVal(o.id, p).trim()),
  );
  const canApply = changed > 0 && !missingParams;

  function apply() {
    const desired = catalog.map((o) => o.id).filter((id) => isOn(id));
    const params: Record<string, Record<string, string>> = {};
    for (const o of catalog) {
      if (isOn(o.id) && o.params?.length) {
        params[o.id] = Object.fromEntries(o.params.map((p) => [p.key, paramVal(o.id, p)]));
      }
    }
    onApply(desired, params);
  }

  const nameOf = (id: string) => catalog.find((o) => o.id === id)?.name ?? id;

  // Undo reverses a change event: disable what it enabled, re-enable what it
  // disabled, then apply the resulting set (re-supplying param defaults for any
  // param-option being re-enabled).
  function undoEvent(ev: { added?: string[]; removed?: string[] }) {
    const cur = new Set(server.options ?? []);
    (ev.added ?? []).forEach((a) => cur.delete(a));
    (ev.removed ?? []).forEach((r) => cur.add(r));
    const desired = [...cur];
    const params: Record<string, Record<string, string>> = {};
    for (const o of catalog) {
      if (cur.has(o.id) && o.params?.length) {
        params[o.id] = Object.fromEntries(o.params.map((p) => [p.key, paramVal(o.id, p)]));
      }
    }
    onApply(desired, params);
  }

  return (
    <div className="border-t border-line px-4 py-4">
      <p className="text-[12px] text-muted">
        hardening settings — tick to add, untick to remove. changes are applied over SSH (as
        mountabo, with sudo) only after you confirm.
      </p>

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
              <span className="min-w-0">
                <span className="flex items-center gap-2 text-[13px] font-medium text-cream">
                  {o.name}
                  {current.has(o.id) && <span className="text-[10px] text-blue">applied</span>}
                </span>
                <span className="mt-1 block text-[12px] leading-5 text-muted">{o.description}</span>
              </span>
            </label>

            {/* inline params, shown when ticked */}
            {isOn(o.id) && o.params && o.params.length > 0 && (
              <div className="mt-3 grid grid-cols-2 gap-2 pl-7">
                {o.params.map((p) => (
                  <label key={p.key} className="block">
                    <span className="mb-1 block text-[11px] text-muted">{p.label}</span>
                    <input
                      value={paramVal(o.id, p)}
                      onChange={(e) => setParam(o.id, p.key, e.target.value)}
                      placeholder={p.placeholder}
                      className={`w-full rounded-md border bg-surface-2 px-2.5 py-1.5 text-[12px] text-cream placeholder:text-muted focus:outline-none ${
                        paramVal(o.id, p).trim() ? "border-line focus:border-line-strong" : "border-red-500/40"
                      }`}
                    />
                  </label>
                ))}
              </div>
            )}
          </li>
        ))}
      </ul>

      <button
        onClick={apply}
        disabled={!canApply}
        className="mt-3 flex w-full items-center justify-center gap-2 rounded-lg border border-lime/50 bg-lime/[0.06] px-4 py-2.5 text-[12.5px] font-medium text-lime transition-colors hover:bg-lime/[0.12] disabled:cursor-not-allowed disabled:opacity-40"
      >
        {changed === 0
          ? "no changes to apply"
          : missingParams
            ? "fill in the required fields above"
            : `confirm & apply ${changed} change${changed === 1 ? "" : "s"}`}
      </button>

      {(server.options?.length ?? 0) > 0 && (
        <button
          onClick={() => onApply([], {})}
          className="mt-2 w-full rounded-lg border border-line px-4 py-2 text-[12px] text-muted transition-colors hover:border-red-400/50 hover:text-red-300"
        >
          revert all hardening ({server.options?.length} applied)
        </button>
      )}

      <ChangeHistory server={server} nameOf={nameOf} onUndo={undoEvent} />
    </div>
  );
}

function ChangeHistory({
  server,
  nameOf,
  onUndo,
}: {
  server: ServerView;
  nameOf: (id: string) => string;
  onUndo: (ev: { added?: string[]; removed?: string[] }) => void;
}) {
  const events = (server.history ?? []).slice().reverse().slice(0, 8);
  if (events.length === 0) return null;

  return (
    <div className="mt-5 border-t border-line pt-3">
      <p className="label">change history</p>
      <ul className="mt-2 space-y-1.5">
        {events.map((ev, i) => {
          const reversible = ev.status === "applied" && ((ev.added?.length ?? 0) + (ev.removed?.length ?? 0)) > 0;
          return (
            <li key={i} className="flex items-center gap-3 text-[12px]">
              <span className="w-28 shrink-0 text-faint">{new Date(ev.at).toLocaleString()}</span>
              <span className="min-w-0 flex-1 truncate">
                {(ev.added ?? []).map((id) => (
                  <span key={id} className="text-lime">
                    +{nameOf(id)}{" "}
                  </span>
                ))}
                {(ev.removed ?? []).map((id) => (
                  <span key={id} className="text-muted">
                    −{nameOf(id)}{" "}
                  </span>
                ))}
              </span>
              {ev.status === "failed" && <span className="text-[11px] text-red-300">failed</span>}
              {reversible && (
                <button
                  onClick={() => onUndo(ev)}
                  className="shrink-0 rounded border border-line px-2 py-0.5 text-[11px] text-muted transition-colors hover:border-lime/50 hover:text-lime"
                >
                  undo
                </button>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
