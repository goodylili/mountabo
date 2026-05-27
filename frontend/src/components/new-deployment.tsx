"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Badge } from "@/components/badge";
import { ServerAvatar } from "@/components/server-avatar";
import {
  ArrowRight,
  Branch,
  Docker,
  GithubMark,
  Lock,
  Plus,
  Refresh,
  Search,
  Shield,
  Trash,
} from "@/components/icons";
import { AddServerForm } from "@/components/add-server-form";
import { OwnerDropdown } from "@/components/owner-dropdown";
import { ServerOptions } from "@/components/server-options";
import { ServerDomains, type DomainFormValue } from "@/components/server-domains";
import { ServerSelect } from "@/components/server-select";
import { StreamLog } from "@/components/stream-log";
import { ConfirmAction } from "@/components/confirm-action";
import type { Source } from "@/lib/data";
import { clearCachedRepos, fetchRepos, readCachedRepos, writeCachedRepos } from "@/lib/repo-cache";
import { type DomainPreview, fetchDomainPreview } from "@/lib/domain-preview";
import type { Domain, ServerStatus, ServerView, SetupOption } from "@/lib/servers";

const statusTone: Record<ServerStatus, "blue" | "lime" | "gray" | "red"> = {
  ready: "blue",
  setting_up: "lime",
  probed: "gray",
  failed: "red",
};

export function NewDeployment({
  servers,
  account,
  stamp,
}: {
  servers: ServerView[];
  account: string | null;
  stamp: string;
}) {
  const router = useRouter();
  // Repositories load in the browser from a 12 hour localStorage cache (keyed by
  // account), falling back to /api/repos on a miss. reposReady gates the empty
  // state so the list shows "loading" instead of "no repositories" while fetching.
  const [sources, setSources] = useState<Source[]>([]);
  const [reposReady, setReposReady] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [clearing, setClearing] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState("");
  const [source, setSource] = useState<string | null>(null);
  const [server, setServer] = useState<string | null>(null);
  const [serverList, setServerList] = useState<ServerView[]>(servers);
  const [view, setView] = useState<"repos" | "servers">("repos");
  const [ownerFilter, setOwnerFilter] = useState<string | null>(null);
  const [visibility, setVisibility] = useState<"all" | "public" | "private">("all");
  const [container, setContainer] = useState<"all" | "compose" | "docker" | "none">("all");
  const [showAddServer, setShowAddServer] = useState(false);
  const [setupTarget, setSetupTarget] = useState<ServerView | null>(null);
  const [page, setPage] = useState(0);
  const [catalog, setCatalog] = useState<SetupOption[]>([]);
  const [applyTarget, setApplyTarget] = useState<{
    server: ServerView;
    desired: string[];
    params: Record<string, Record<string, string>>;
  } | null>(null);
  const [domainTarget, setDomainTarget] = useState<
    | { server: ServerView; mode: "add"; value: DomainFormValue }
    | { server: ServerView; mode: "remove"; host: string }
    | null
  >(null);
  // Confirmation gates: every server-touching action opens one of these first
  // and only fires (sets the matching *Target above, or runs the delete) once
  // the operator confirms. Nothing reaches a server unconfirmed.
  const [confirmSetup, setConfirmSetup] = useState<ServerView | null>(null);
  const [confirmApply, setConfirmApply] = useState<{
    server: ServerView;
    desired: string[];
    params: Record<string, Record<string, string>>;
  } | null>(null);
  const [confirmDomain, setConfirmDomain] = useState<
    | { server: ServerView; mode: "add"; value: DomainFormValue }
    | { server: ServerView; mode: "remove"; host: string }
    | null
  >(null);
  // The fetched add-domain preview (nginx config + script) for the open gate,
  // and a flag while it loads. Removing a domain needs no preview.
  const [domainPreview, setDomainPreview] = useState<DomainPreview | null>(null);
  const [domainPreviewLoading, setDomainPreviewLoading] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<ServerView | null>(null);

  function setServerStatus(id: string, status: ServerStatus) {
    setServerList((prev) => prev.map((s) => (s.id === id ? { ...s, status } : s)));
  }

  // Resolves a hardening option id to its catalog display name for the apply
  // confirmation, falling back to the id when the catalog hasn't loaded.
  const optionName = (id: string) => catalog.find((o) => o.id === id)?.name ?? id;

  // After a successful apply, update the server's options AND append a change
  // event locally so the history timeline and undo stay current without a refetch.
  function recordApply(target: ServerView, desired: string[]) {
    const prev = target.options ?? [];
    const prevSet = new Set(prev);
    const desiredSet = new Set(desired);
    const added = desired.filter((id) => !prevSet.has(id));
    const removed = prev.filter((id) => !desiredSet.has(id));
    const event = { at: new Date().toISOString(), added, removed, status: "applied" };
    setServerList((list) =>
      list.map((s) =>
        s.id === target.id ? { ...s, options: desired, history: [...(s.history ?? []), event] } : s,
      ),
    );
  }

  // After a domain is added (or updated), reflect it locally so the list and the
  // www badge stay current without a refetch; re-adding the same host replaces it.
  function recordDomainAdd(target: ServerView, value: DomainFormValue) {
    const entry: Domain = {
      host: value.host,
      aliases: value.aliases,
      upstream: value.upstream,
      email: value.email || undefined,
      createdAt: new Date().toISOString(),
    };
    setServerList((list) =>
      list.map((s) =>
        s.id === target.id
          ? { ...s, domains: [...(s.domains ?? []).filter((d) => d.host !== entry.host), entry] }
          : s,
      ),
    );
  }

  function recordDomainRemove(target: ServerView, host: string) {
    setServerList((list) =>
      list.map((s) =>
        s.id === target.id ? { ...s, domains: (s.domains ?? []).filter((d) => d.host !== host) } : s,
      ),
    );
  }

  // Removes the server: destroys its keychain secrets (mountabo key + any
  // retained root password) on the backend, then drops it from the list. Only
  // ever called from the confirmation gate. Clears the selection if it was this
  // server so the deploy bar doesn't point at a server that no longer exists.
  async function deleteServer(target: ServerView) {
    try {
      await fetch(`/api/servers/${target.id}`, { method: "DELETE" });
    } catch {
      // best effort; the list still drops it so the operator isn't stuck
    }
    setServerList((list) => list.filter((s) => s.id !== target.id));
    setServer((cur) => (cur === target.id ? null : cur));
  }

  // Load repositories on mount: serve a fresh cache instantly, otherwise fetch
  // and cache. Re-runs if the connected account changes (cache is per account).
  useEffect(() => {
    let active = true;
    async function load() {
      if (!account) {
        if (active) {
          setSources([]);
          setReposReady(true);
        }
        return;
      }
      const cached = readCachedRepos(account);
      if (cached) {
        if (active) {
          setSources(cached.repos);
          setReposReady(true);
        }
        return;
      }
      try {
        const repos = await fetchRepos();
        if (!active) return;
        setSources(repos);
        writeCachedRepos(account, repos);
      } catch {
        // a failed first load leaves an honest empty state, not fabricated repos
      } finally {
        if (active) setReposReady(true);
      }
    }
    void load();
    return () => {
      active = false;
    };
  }, [account]);

  // The refresh button forces a fresh listing past the cache, for when the user
  // has created a repository or been granted access to a new one. A failed
  // refresh keeps the current list rather than blanking it.
  async function refreshRepos() {
    if (!account || refreshing) return;
    setRefreshing(true);
    try {
      const repos = await fetchRepos();
      setSources(repos);
      writeCachedRepos(account, repos);
    } catch {
      // keep the current list
    } finally {
      setRefreshing(false);
    }
  }

  // The clear cache button purges the stored repository cache and reloads the
  // list from GitHub, for when the cached listing is stale or wrong. Unlike
  // refresh, it reloads from scratch (the list blanks to its loading state) so
  // the cleared, refilled result is visibly fresh, then re-caches it.
  async function clearCache() {
    if (!account || clearing) return;
    setClearing(true);
    clearCachedRepos();
    setReposReady(false);
    setSources([]);
    try {
      const repos = await fetchRepos();
      setSources(repos);
      writeCachedRepos(account, repos);
    } catch {
      // leave the list empty; the next load will retry
    } finally {
      setReposReady(true);
      setClearing(false);
    }
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

  // When the add-domain gate opens, fetch the exact nginx config + script the
  // backend would write, so the operator sees precisely what will run before
  // anything touches the server. Removing a domain needs no preview.
  useEffect(() => {
    if (!confirmDomain || confirmDomain.mode !== "add") {
      setDomainPreview(null);
      setDomainPreviewLoading(false);
      return;
    }
    const ctrl = new AbortController();
    setDomainPreview(null);
    setDomainPreviewLoading(true);
    fetchDomainPreview(confirmDomain.value, ctrl.signal)
      .then((res) => {
        if (ctrl.signal.aborted) return;
        if (!("error" in res)) setDomainPreview(res);
      })
      .catch(() => {}) // includes AbortError; the gate falls back to a step list
      .finally(() => {
        if (!ctrl.signal.aborted) setDomainPreviewLoading(false);
      });
    return () => ctrl.abort();
  }, [confirmDomain]);

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
    if (visibility !== "all") list = list.filter((s) => (visibility === "private" ? s.private : !s.private));
    if (container !== "all") list = list.filter((s) => (s.kind ?? "none") === container);
    if (q) list = list.filter((s) => `${s.name} ${s.language}`.toLowerCase().includes(q));
    return list;
  }, [sources, q, ownerFilter, visibility, container]);
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
    <main className="mx-auto flex w-full max-w-[1100px] flex-1 flex-col px-4 pb-10 pt-10 sm:px-6 sm:pt-16 lg:px-8">
      {/* hero */}
      <div className="rise">
        <p className="label">
          new deployment · {stamp}
        </p>
        <h1 className="mt-6 text-4xl font-extrabold leading-[1.02] tracking-tight text-cream sm:text-5xl sm:leading-[0.98] lg:text-6xl">
          deploy something
          <br />
          of your <span className="italic text-lime">own</span> by{" "}
          <span className="italic text-lime">yourself.</span>
        </h1>
        <p className="mt-6 max-w-2xl text-[16px] leading-8 text-body">
          Vercel Style Frontend for your VPS. pick a source, point it at a server. mountabo writes{" "}
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

      {/* view tabs: repositories / servers, each full-width */}
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
            <span className="flex items-center gap-4">
              <button
                onClick={clearCache}
                disabled={clearing || refreshing || !account}
                className="flex items-center gap-1.5 text-[12px] text-muted transition-colors hover:text-cream disabled:opacity-60"
              >
                <Trash /> {clearing ? "clearing…" : "clear cache"}
              </button>
              <button
                onClick={refreshRepos}
                disabled={refreshing || clearing || !account}
                className="flex items-center gap-1.5 text-[12px] text-lime transition-colors hover:text-cream disabled:opacity-60"
              >
                <Refresh className={refreshing ? "animate-spin" : ""} /> {refreshing ? "refreshing…" : "refresh"}
              </button>
            </span>
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

          <div className="flex flex-wrap items-center gap-x-6 gap-y-2 border-b border-line px-5 py-2.5">
            <span className="flex items-center gap-2">
              <span className="text-[11px] text-faint">visibility</span>
              <FilterChips
                value={visibility}
                onChange={(v) => {
                  setVisibility(v);
                  setPage(0);
                }}
                options={[
                  { v: "all", label: "all" },
                  { v: "public", label: "public" },
                  { v: "private", label: "private" },
                ]}
              />
            </span>
            <span className="flex items-center gap-2">
              <span className="text-[11px] text-faint">container</span>
              <FilterChips
                value={container}
                onChange={(v) => {
                  setContainer(v);
                  setPage(0);
                }}
                options={[
                  { v: "all", label: "all" },
                  { v: "compose", label: "compose" },
                  { v: "docker", label: "docker" },
                  { v: "none", label: "non-docker" },
                ]}
              />
            </span>
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
                      {s.hasDocker && (
                        <span title="has a Dockerfile or Compose file">
                          <Docker className="text-[#2496ED]" />
                        </span>
                      )}
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
                ) : !reposReady ? (
                  <>loading repositories…</>
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
                  className={`rounded-lg border bg-surface-2 transition-colors ${
                    active ? "border-lime/60" : "border-line hover:border-line-strong"
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
                        onClick={() => setConfirmSetup(s)}
                        disabled={s.status === "setting_up"}
                        className="rounded-md border border-lime/40 px-3 py-1.5 text-[12px] text-lime transition-colors hover:bg-lime/10 disabled:opacity-50"
                      >
                        {s.status === "failed" ? "retry setup" : s.status === "setting_up" ? "setting up…" : "set up"}
                      </button>
                    )}
                    <button
                      onClick={() => setConfirmDelete(s)}
                      disabled={s.status === "setting_up"}
                      title={`remove ${s.name} from mountabo`}
                      aria-label={`remove ${s.name}`}
                      className="shrink-0 rounded-md border border-line px-2.5 py-1.5 text-[12px] text-muted transition-colors hover:border-red-400/50 hover:text-red-300 disabled:opacity-50"
                    >
                      <Trash />
                    </button>
                  </div>
                  {active && s.status === "ready" && (
                    <>
                      <ServerDomains
                        key={`dom:${(s.domains ?? []).map((d) => d.host).join(",")}`}
                        server={s}
                        onAdd={(value) => setConfirmDomain({ server: s, mode: "add", value })}
                        onRemove={(host) => setConfirmDomain({ server: s, mode: "remove", host })}
                      />
                      <ServerOptions
                        key={`${s.id}:${(s.options ?? []).join(",")}`}
                        server={s}
                        catalog={catalog}
                        onApply={(desired, params) => setConfirmApply({ server: s, desired, params })}
                      />
                    </>
                  )}
                </li>
              );
            })}

            {/* Adding a server needs its ip + root password (to bootstrap it).
                Once a server is selected, that's irrelevant, deploys use its SSH
                key, so the onboarding fields are hidden. */}
            {!server && (
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
            )}
          </ul>
        </section>
        )}
      </div>

      {/* deploy bar: pick a target server, then head to the deployment page */}
      {source && (
        <div className="rise mt-5 flex flex-col gap-3 rounded-xl border border-lime/50 bg-lime/[0.06] px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
          <span className="flex min-w-0 items-center gap-2 text-[14px] text-cream">
            <span className="shrink-0 text-lime">→ deploy</span>
            <span className="truncate">{source}</span>
          </span>
          <div className="flex w-full flex-col gap-3 sm:w-auto sm:flex-row sm:items-center">
            <ServerSelect servers={readyServers} value={server} onChange={setServer} />
            <Link
              href={configureHref}
              aria-disabled={!ready}
              tabIndex={ready ? 0 : -1}
              className={`flex items-center justify-center gap-2 rounded-md px-4 py-2.5 text-[13px] font-bold transition-transform sm:py-2 ${
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
        url={applyUrl(applyTarget.server.id, applyTarget.desired, applyTarget.params)}
        onDone={(ok) => {
          if (ok) recordApply(applyTarget.server, applyTarget.desired);
        }}
        onClose={() => setApplyTarget(null)}
      />
    )}
    {domainTarget && (
      <StreamLog
        title={
          domainTarget.mode === "add"
            ? `adding ${domainTarget.value.host}`
            : `removing ${domainTarget.host}`
        }
        subtitle={domainTarget.server.ip}
        timezone={domainTarget.server.timezone}
        url={
          domainTarget.mode === "add"
            ? addDomainUrl(domainTarget.server.id, domainTarget.value)
            : removeDomainUrl(domainTarget.server.id, domainTarget.host)
        }
        onDone={(ok) => {
          if (!ok) return;
          if (domainTarget.mode === "add") recordDomainAdd(domainTarget.server, domainTarget.value);
          else recordDomainRemove(domainTarget.server, domainTarget.host);
        }}
        onClose={() => setDomainTarget(null)}
      />
    )}

    {/* Confirmation gates: each opens before the matching action runs, and only
        a confirm sets the *Target (or runs the delete) that touches the server. */}
    {confirmSetup && (
      <ConfirmAction
        title={`set up ${confirmSetup.name}`}
        subtitle={confirmSetup.ip}
        summary={
          <>
            mountabo will connect to <span className="text-cream">{confirmSetup.ip}</span> as root once
            and prepare it for deployments. it creates an unprivileged{" "}
            <span className="text-cream">mountabo</span> user, generates and installs a dedicated
            ed25519 deploy key, and applies the base hardening. you can add or remove hardening later.
            nothing runs until you confirm.
          </>
        }
        steps={[
          "connect to the server as root over SSH using the password you provided",
          "create the unprivileged mountabo user with sudo and a locked password",
          "generate an ed25519 deploy key and install its public key for the mountabo user",
          "store the private key in your OS keychain, never on the server beyond its authorized_keys",
          "apply the base hardening and confirm mountabo can log in with the new key",
        ]}
        confirmLabel={confirmSetup.status === "failed" ? "retry setup" : "set up server"}
        onConfirm={() => {
          setSetupTarget(confirmSetup);
          setConfirmSetup(null);
        }}
        onCancel={() => setConfirmSetup(null)}
      />
    )}

    {confirmApply && (
      <ConfirmAction
        title={`apply settings to ${confirmApply.server.name}`}
        subtitle={confirmApply.server.ip}
        summary={(() => {
          const prev = new Set(confirmApply.server.options ?? []);
          const desired = new Set(confirmApply.desired);
          const added = confirmApply.desired.filter((id) => !prev.has(id));
          const removed = (confirmApply.server.options ?? []).filter((id) => !desired.has(id));
          return (
            <>
              mountabo will connect to <span className="text-cream">{confirmApply.server.name}</span> as
              the mountabo user (with sudo) and change its hardening:{" "}
              {added.length > 0 && (
                <span className="text-lime">install {added.map(optionName).join(", ")}</span>
              )}
              {added.length > 0 && removed.length > 0 && ", "}
              {removed.length > 0 && (
                <span className="text-muted">remove {removed.map(optionName).join(", ")}</span>
              )}
              {added.length === 0 && removed.length === 0 && "no changes"}. nothing runs until you
              confirm.
            </>
          );
        })()}
        steps={(() => {
          const prev = new Set(confirmApply.server.options ?? []);
          const desired = new Set(confirmApply.desired);
          const added = confirmApply.desired.filter((id) => !prev.has(id));
          const removed = (confirmApply.server.options ?? []).filter((id) => !desired.has(id));
          const lines = ["connect to the server as the mountabo user over SSH (with sudo)"];
          for (const id of added) lines.push(`install and enable: ${optionName(id)}`);
          for (const id of removed) lines.push(`disable and remove: ${optionName(id)}`);
          return lines;
        })()}
        confirmLabel="apply settings"
        onConfirm={() => {
          setApplyTarget(confirmApply);
          setConfirmApply(null);
        }}
        onCancel={() => setConfirmApply(null)}
      />
    )}

    {confirmDomain && confirmDomain.mode === "add" && (
      <ConfirmAction
        title={`add ${confirmDomain.value.host}`}
        subtitle={confirmDomain.server.ip}
        summary={
          <>
            mountabo will configure <span className="text-cream">{confirmDomain.server.name}</span> to
            front <span className="text-cream">{confirmDomain.value.host}</span> on https, proxying to
            your app on port <span className="text-cream">{confirmDomain.value.upstream}</span>. it
            installs nginx if needed, writes the vhost below, and obtains a Let&apos;s Encrypt
            certificate over http, so the domain must already point at{" "}
            <span className="text-cream">{confirmDomain.server.ip}</span>. nothing runs until you
            confirm.
          </>
        }
        loading={domainPreviewLoading}
        steps={
          domainPreview
            ? `# ${domainPreview.sitePath} (http)\n${domainPreview.httpConfig}\n\n# ${domainPreview.sitePath} (https)\n${domainPreview.tlsConfig}\n\n# setup script\n${domainPreview.script}`
            : domainPreviewLoading
              ? undefined
              : [
                  "connect to the server as the mountabo user over SSH (with sudo)",
                  "install nginx if it is not already present",
                  `write the nginx vhost for ${confirmDomain.value.host}`,
                  "obtain a Let's Encrypt certificate over http and enable https",
                  "reload nginx so the domain serves over https",
                ]
        }
        confirmLabel={`add ${confirmDomain.value.host}`}
        onConfirm={() => {
          setDomainTarget(confirmDomain);
          setConfirmDomain(null);
        }}
        onCancel={() => setConfirmDomain(null)}
      />
    )}

    {confirmDomain && confirmDomain.mode === "remove" && (
      <ConfirmAction
        title={`remove ${confirmDomain.host}`}
        subtitle={confirmDomain.server.ip}
        destructive
        summary={
          <>
            mountabo will stop fronting <span className="text-cream">{confirmDomain.host}</span> on{" "}
            <span className="text-cream">{confirmDomain.server.name}</span>: it removes the nginx vhost
            and its certificate, then reloads nginx. your app keeps running on its local port. nothing
            runs until you confirm.
          </>
        }
        steps={[
          "connect to the server as the mountabo user over SSH (with sudo)",
          `remove the nginx vhost for ${confirmDomain.host}`,
          "delete the Let's Encrypt certificate for the domain",
          "reload nginx so it stops serving the domain",
        ]}
        confirmLabel="remove domain"
        onConfirm={() => {
          setDomainTarget(confirmDomain);
          setConfirmDomain(null);
        }}
        onCancel={() => setConfirmDomain(null)}
      />
    )}

    {confirmDelete && (
      <ConfirmAction
        title={`remove ${confirmDelete.name}`}
        subtitle={confirmDelete.ip}
        destructive
        summary={
          <>
            mountabo will remove <span className="text-cream">{confirmDelete.name}</span> from mountabo
            and destroy its stored secrets. this revokes mountabo&apos;s access to the server. it does
            not stop or delete anything already deployed on the server itself. this cannot be undone.
          </>
        }
        steps={[
          "destroy the server's mountabo deploy key in your OS keychain",
          "destroy any retained root password for the server in your OS keychain",
          "delete the server from mountabo's records, along with its domains and history",
        ]}
        confirmLabel="remove server"
        onConfirm={() => {
          void deleteServer(confirmDelete);
          setConfirmDelete(null);
        }}
        onCancel={() => setConfirmDelete(null)}
      />
    )}
    </>
  );
}

// Builds the add-domain SSE URL: ?host=&upstream=&aliases=&email=&staging=.
function addDomainUrl(serverId: string, v: DomainFormValue): string {
  const qs = new URLSearchParams();
  qs.set("host", v.host);
  if (v.upstream) qs.set("upstream", v.upstream);
  if (v.aliases.length) qs.set("aliases", v.aliases.join(","));
  if (v.email) qs.set("email", v.email);
  if (v.staging) qs.set("staging", "1");
  return `/api/servers/${serverId}/domains/add?${qs.toString()}`;
}

function removeDomainUrl(serverId: string, host: string): string {
  return `/api/servers/${serverId}/domains/remove?host=${encodeURIComponent(host)}`;
}

// Builds the apply-options SSE URL: ?set=<ids>&param.<id>.<key>=<value>.
function applyUrl(
  serverId: string,
  desired: string[],
  params: Record<string, Record<string, string>>,
): string {
  const qs = new URLSearchParams();
  qs.set("set", desired.join(","));
  for (const [optId, kv] of Object.entries(params)) {
    for (const [key, val] of Object.entries(kv)) {
      qs.set(`param.${optId}.${key}`, val);
    }
  }
  return `/api/servers/${serverId}/options?${qs.toString()}`;
}

// FilterChips is a small segmented control: one active value out of a few.
function FilterChips<T extends string>({
  value,
  onChange,
  options,
}: {
  value: T;
  onChange: (v: T) => void;
  options: { v: T; label: string }[];
}) {
  return (
    <span className="flex items-center gap-1">
      {options.map((o) => (
        <button
          key={o.v}
          onClick={() => onChange(o.v)}
          className={`rounded-md px-2 py-1 text-[11px] transition-colors ${
            value === o.v ? "bg-surface-2 text-cream" : "text-muted hover:text-cream"
          }`}
        >
          {o.label}
        </button>
      ))}
    </span>
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
