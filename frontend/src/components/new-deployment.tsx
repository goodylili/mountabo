"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Badge } from "@/components/badge";
import { ServerAvatar } from "@/components/server-avatar";
import {
  ArrowRight,
  Branch,
  GithubMark,
  Lock,
  Plus,
  Refresh,
  Search,
  Shield,
} from "@/components/icons";
import { AddServerForm } from "@/components/add-server-form";
import { OwnerDropdown } from "@/components/owner-dropdown";
import { ServerOptions } from "@/components/server-options";
import { StreamLog } from "@/components/stream-log";
import type { Source } from "@/lib/data";
import type { ServerStatus, ServerView, SetupOption } from "@/lib/servers";

const statusTone: Record<ServerStatus, "blue" | "lime" | "gray" | "red"> = {
  ready: "blue",
  setting_up: "lime",
  probed: "gray",
  failed: "red",
};

export function NewDeployment({
  sources,
  servers,
  account,
  stamp,
}: {
  sources: Source[];
  servers: ServerView[];
  account: string | null;
  stamp: string;
}) {
  const router = useRouter();
  const searchRef = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState("");
  const [source, setSource] = useState<string | null>(null);
  const [server, setServer] = useState<string | null>(null);
  const [serverList, setServerList] = useState<ServerView[]>(servers);
  const [view, setView] = useState<"repos" | "servers">("repos");
  const [ownerFilter, setOwnerFilter] = useState<string | null>(null);
  const [showAddServer, setShowAddServer] = useState(false);
  const [setupTarget, setSetupTarget] = useState<ServerView | null>(null);
  const [page, setPage] = useState(0);
  const [catalog, setCatalog] = useState<SetupOption[]>([]);
  const [applyTarget, setApplyTarget] = useState<{ server: ServerView; desired: string[] } | null>(null);

  function setServerStatus(id: string, status: ServerStatus) {
    setServerList((prev) => prev.map((s) => (s.id === id ? { ...s, status } : s)));
  }

  function updateServerOptions(id: string, options: string[]) {
    setServerList((prev) => prev.map((s) => (s.id === id ? { ...s, options } : s)));
  }

  // The hardening catalog (id/name/description) for the per-server toggles.
  useEffect(() => {
    let active = true;
    fetch("/api/servers/options")
      .then((r) => r.json())
      .then((o) => {
        if (active) setCatalog(o as SetupOption[]);
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, []);

  // ⌘K / ctrl-K focuses the palette; Enter continues when both are picked.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        searchRef.current?.focus();
      }
      if (e.key === "Enter" && source && server && document.activeElement !== searchRef.current) {
        router.push(`/configure?repo=${encodeURIComponent(source)}&server=${server}`);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [source, server, router]);

  const q = query.trim().toLowerCase();
  // Distinct repo owners (account + organizations) for the owner dropdown.
  const owners = useMemo(() => Array.from(new Set(sources.map((s) => s.owner))).sort(), [sources]);
  const filteredSources = useMemo(() => {
    let list = sources;
    if (ownerFilter) list = list.filter((s) => s.owner === ownerFilter);
    if (q) list = list.filter((s) => `${s.name} ${s.language}`.toLowerCase().includes(q));
    return list;
  }, [sources, q, ownerFilter]);
  // Repo lists can be large (hundreds), so paginate the sources client-side.
  // The page resets to 0 in the search handler whenever the filter changes.
  const SOURCES_PER_PAGE = 8;
  const pageCount = Math.max(1, Math.ceil(filteredSources.length / SOURCES_PER_PAGE));
  const safePage = Math.min(page, pageCount - 1);
  const pagedSources = filteredSources.slice(
    safePage * SOURCES_PER_PAGE,
    safePage * SOURCES_PER_PAGE + SOURCES_PER_PAGE,
  );

  const filteredServers = useMemo(
    () => (q ? serverList.filter((s) => `${s.name} ${s.ip} ${s.specs.os}`.toLowerCase().includes(q)) : serverList),
    [serverList, q],
  );

  const readyServers = serverList.filter((s) => s.status === "ready");
  const ready = Boolean(source && server);
  const configureHref = ready
    ? `/configure?repo=${encodeURIComponent(source as string)}&server=${server}`
    : "#";

  return (
    <>
    <main className="mx-auto flex w-full max-w-[1400px] flex-1 flex-col px-8 pb-10 pt-16">
      {/* hero */}
      <div className="rise">
        <p className="label">
          new deployment · {stamp}
        </p>
        <h1 className="mt-6 text-6xl font-extrabold leading-[0.98] tracking-tight text-cream">
          deploy something
          <br />
          of your <span className="italic text-lime">own.</span>
        </h1>
        <p className="mt-6 max-w-2xl text-[15px] leading-7 text-body">
          pick a source, point it at a server. mountabo writes{" "}
          <span className="text-cream">.github/workflows/mountabo-deploy.yml</span> (or a local
          script) and steps out of the way. no middleman, no surprises, nothing leaves your machine.
        </p>
      </div>

      {/* command palette */}
      <div className="rise mt-10" style={{ animationDelay: "70ms" }}>
        <label className="flex items-center gap-3 rounded-xl border border-line bg-surface px-4 py-3.5 transition-colors focus-within:border-line-strong">
          <Search className="text-muted" />
          <input
            ref={searchRef}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setPage(0);
            }}
            placeholder="filter sources and servers, or jump anywhere…"
            className="flex-1 bg-transparent text-[14px] text-cream placeholder:text-muted focus:outline-none"
          />
          <kbd className="rounded-md border border-line bg-surface-2 px-2 py-0.5 text-[11px] text-muted">⌘</kbd>
          <kbd className="rounded-md border border-line bg-surface-2 px-2 py-0.5 text-[11px] text-muted">K</kbd>
        </label>
      </div>

      {/* view tabs — repositories / servers, each full-width */}
      <div className="rise mt-6 flex items-center gap-1" style={{ animationDelay: "120ms" }}>
        <ViewTab active={view === "repos"} onClick={() => setView("repos")} count={filteredSources.length}>
          repositories
        </ViewTab>
        <ViewTab active={view === "servers"} onClick={() => setView("servers")} count={serverList.length}>
          servers
        </ViewTab>
      </div>

      {/* active view */}
      <div className="rise mt-4 flex flex-1 flex-col" style={{ animationDelay: "140ms" }}>
        {view === "repos" && (
        <section className="flex flex-col rounded-xl border border-line bg-surface">
          <div className="flex items-center justify-between border-b border-line px-5 py-4">
            <span className="flex items-center gap-2">
              <span className="label">sources</span>
              <span className="rounded border border-line px-1.5 py-0.5 text-[11px] text-muted">
                {String(filteredSources.length).padStart(2, "0")}
              </span>
            </span>
            <button className="flex items-center gap-1.5 text-[12px] text-lime transition-colors hover:text-cream">
              <Refresh /> refresh
            </button>
          </div>

          <div className="flex items-center justify-between border-b border-line px-5 py-3 text-[13px]">
            <div className="flex items-center gap-2">
              <span className="flex items-center gap-2 rounded-md border border-line bg-surface-2 px-2.5 py-1.5 text-muted">
                <GithubMark /> github
              </span>
              <span className="text-faint">›</span>
              <OwnerDropdown
                owners={owners}
                value={ownerFilter}
                account={account}
                onChange={(o) => {
                  setOwnerFilter(o);
                  setPage(0);
                }}
              />
            </div>
            <span className="text-muted">↳ {filteredSources.length} visible</span>
          </div>

          <ul className="flex-1">
            {pagedSources.map((s) => {
              const full = `${s.owner}/${s.name}`;
              const active = source === full;
              return (
                <li
                  key={full}
                  className={`flex items-center gap-3 px-5 transition-colors ${
                    active ? "bg-lime/[0.08]" : "hover:bg-surface-hover"
                  }`}
                >
                  <div className="min-w-0 flex-1 py-3.5">
                    <a
                      href={`https://github.com/${full}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      title={`open ${full} on GitHub`}
                      className="inline-flex items-center gap-2 text-[15px] font-medium text-cream transition-colors hover:text-lime hover:underline"
                    >
                      {s.name}
                      {s.private && <Lock className="text-muted" />}
                    </a>
                    <span className="mt-1 flex items-center gap-2 text-[12px] text-muted">
                      <Branch className={s.branchAccent ? "text-lime" : "text-muted"} />
                      <span className={s.branchAccent ? "text-lime" : ""}>{s.branch}</span>
                      <span className="text-faint">·</span>
                      {s.updated}
                      <span className="text-faint">·</span>
                      {s.language}
                      {s.loc && (
                        <>
                          <span className="text-faint">·</span>
                          {s.loc}
                        </>
                      )}
                    </span>
                  </div>
                  <button
                    onClick={() => setSource(full)}
                    className={`flex shrink-0 items-center gap-1.5 rounded-md border px-3 py-1.5 text-[12px] transition-colors ${
                      active
                        ? "border-lime/60 bg-lime/[0.08] text-lime"
                        : "border-line text-muted hover:border-lime/50 hover:text-lime"
                    }`}
                  >
                    {active ? (
                      <>
                        selected <ArrowRight className="text-lime" />
                      </>
                    ) : (
                      "select"
                    )}
                  </button>
                </li>
              );
            })}
            {filteredSources.length === 0 && (
              <li className="px-5 py-8 text-center text-[13px] text-muted">
                {!account ? (
                  <>
                    connect github to list your repositories.{" "}
                    <Link href="/connect" className="text-lime hover:text-cream">
                      connect →
                    </Link>
                  </>
                ) : q ? (
                  <>no sources match “{query}”.</>
                ) : (
                  <>no repositories found.</>
                )}
              </li>
            )}
          </ul>

          {pageCount > 1 && (
            <div className="flex items-center justify-between border-t border-line px-5 py-3 text-[12px]">
              <button
                onClick={() => setPage((p) => Math.max(0, p - 1))}
                disabled={safePage === 0}
                className="rounded-md border border-line px-2.5 py-1 text-muted transition-colors hover:text-cream disabled:opacity-40"
              >
                ‹ prev
              </button>
              <span className="text-muted">
                page {safePage + 1} of {pageCount}
              </span>
              <button
                onClick={() => setPage((p) => Math.min(pageCount - 1, p + 1))}
                disabled={safePage >= pageCount - 1}
                className="rounded-md border border-line px-2.5 py-1 text-muted transition-colors hover:text-cream disabled:opacity-40"
              >
                next ›
              </button>
            </div>
          )}
        </section>
        )}

        {view === "servers" && (
        <section className="flex flex-col rounded-xl border border-line bg-surface">
          <div className="flex items-center justify-between border-b border-line px-5 py-4">
            <span className="flex items-center gap-2">
              <span className="label">servers</span>
              <span className="rounded border border-line px-1.5 py-0.5 text-[11px] text-muted">
                {String(filteredServers.length).padStart(2, "0")}
              </span>
            </span>
          </div>

          <ul className="flex flex-1 flex-col gap-3 p-4">
            {filteredServers.map((s) => {
              const active = server === s.id;
              const ramGB = s.specs.memoryMB ? `${Math.round(s.specs.memoryMB / 1024)} gb` : "-";
              return (
                <li
                  key={s.id}
                  className={`rounded-lg border transition-colors ${
                    active ? "border-lime/60 bg-lime/[0.05]" : "border-line bg-surface-2 hover:border-line-strong"
                  }`}
                >
                  <div className="flex items-center gap-4 px-4 py-3.5">
                    <button onClick={() => setServer(s.id)} className="flex flex-1 items-center gap-4 text-left">
                      <ServerAvatar seed={s.name} />
                      <div className="flex-1">
                        <span className="flex items-center gap-2.5 text-[15px] font-medium text-cream">
                          {s.name}
                          <Badge tone={statusTone[s.status]} dot>
                            {s.status.replace("_", " ")}
                          </Badge>
                        </span>
                        <span className="mt-1 block text-[12px] text-muted">
                          {s.ip} · {s.specs.os || "unknown os"} · {s.specs.cpuCores || "-"} vcpu · {ramGB} ·{" "}
                          {s.specs.diskGB ? `${s.specs.diskGB} gb disk` : "-"}
                        </span>
                      </div>
                    </button>
                    {s.status === "ready" ? (
                      <span className="text-[12px] text-blue">✓ ready</span>
                    ) : (
                      <button
                        onClick={() => setSetupTarget(s)}
                        disabled={s.status === "setting_up"}
                        className="rounded-md border border-lime/40 px-3 py-1.5 text-[12px] text-lime transition-colors hover:bg-lime/10 disabled:opacity-50"
                      >
                        {s.status === "failed" ? "retry setup" : s.status === "setting_up" ? "setting up…" : "set up"}
                      </button>
                    )}
                  </div>
                  {active && s.status === "ready" && (
                    <ServerOptions
                      key={`${s.id}:${(s.options ?? []).join(",")}`}
                      server={s}
                      catalog={catalog}
                      onApply={(desired) => setApplyTarget({ server: s, desired })}
                    />
                  )}
                </li>
              );
            })}

            <li>
              {showAddServer ? (
                <AddServerForm
                  onAdded={(srv) => {
                    setServerList((prev) => [...prev, srv]);
                    setShowAddServer(false);
                  }}
                  onCancel={() => setShowAddServer(false)}
                />
              ) : (
                <button
                  onClick={() => setShowAddServer(true)}
                  className="flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-line-strong px-4 py-4 text-[13px] text-muted transition-colors hover:border-lime/50 hover:text-cream"
                >
                  <Plus /> add a server (ip + root password)
                </button>
              )}
            </li>
          </ul>
        </section>
        )}
      </div>

      {/* deploy bar: pick a target server, then head to the deployment page */}
      {source && (
        <div className="rise mt-5 flex flex-col gap-3 rounded-xl border border-lime/50 bg-lime/[0.06] px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
          <span className="flex items-center gap-2 text-[14px] text-cream">
            <span className="text-lime">→ deploy</span>
            {source}
          </span>
          <div className="flex items-center gap-3">
            <select
              value={server ?? ""}
              onChange={(e) => setServer(e.target.value || null)}
              className="rounded-md border border-line bg-surface px-3 py-2 text-[13px] text-cream focus:border-line-strong focus:outline-none"
            >
              <option value="">
                {readyServers.length ? "select a server…" : "no ready servers — set one up first"}
              </option>
              {readyServers.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name} · {s.ip}
                </option>
              ))}
            </select>
            <Link
              href={configureHref}
              aria-disabled={!ready}
              tabIndex={ready ? 0 : -1}
              className={`flex items-center gap-2 rounded-md px-4 py-2 text-[13px] font-bold transition-transform ${
                ready
                  ? "cta-glow bg-lime-fill text-black hover:-translate-y-0.5"
                  : "pointer-events-none border border-line text-muted opacity-50"
              }`}
            >
              deploy <ArrowRight />
            </Link>
          </div>
        </div>
      )}

      {/* footer */}
      <div className="mt-8 flex flex-col gap-4 border-t border-line pt-5 sm:flex-row sm:items-center sm:justify-between">
        <p className="flex items-center gap-2 text-[12px] text-muted">
          <Shield className="text-faint" />
          this stays on your machine. credentials live in your os keychain.
        </p>
        <div className="flex items-center gap-3 text-[12px]">
          <button className="rounded-md border border-line px-3 py-2 text-muted transition-colors hover:border-line-strong hover:text-cream">
            view cli command
          </button>
          <button className="rounded-md border border-line px-3 py-2 text-muted transition-colors hover:border-line-strong hover:text-cream">
            import .yml
          </button>
        </div>
      </div>
    </main>

    {setupTarget && (
      <StreamLog
        title={`setting up ${setupTarget.name}`}
        subtitle={setupTarget.ip}
        timezone={setupTarget.timezone}
        url={`/api/servers/${setupTarget.id}/setup`}
        onDone={(ok) => setServerStatus(setupTarget.id, ok ? "ready" : "failed")}
        onClose={() => setSetupTarget(null)}
      />
    )}
    {applyTarget && (
      <StreamLog
        title={`applying settings to ${applyTarget.server.name}`}
        subtitle={applyTarget.server.ip}
        timezone={applyTarget.server.timezone}
        url={`/api/servers/${applyTarget.server.id}/options?set=${encodeURIComponent(applyTarget.desired.join(","))}`}
        onDone={(ok) => {
          if (ok) updateServerOptions(applyTarget.server.id, applyTarget.desired);
        }}
        onClose={() => setApplyTarget(null)}
      />
    )}
    </>
  );
}

function ViewTab({
  active,
  onClick,
  count,
  children,
}: {
  active: boolean;
  onClick: () => void;
  count: number;
  children: ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      className={`flex items-center gap-2 rounded-lg px-4 py-2 text-[13px] transition-colors ${
        active ? "bg-surface-2 text-cream" : "text-muted hover:text-cream"
      }`}
    >
      {children}
      <span className="rounded border border-line px-1.5 py-0.5 text-[11px] text-muted">
        {String(count).padStart(2, "0")}
      </span>
    </button>
  );
}
