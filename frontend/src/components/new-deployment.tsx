"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { Badge } from "@/components/badge";
import { ServerAvatar } from "@/components/server-avatar";
import {
  ArrowRight,
  Branch,
  ChevronDown,
  GithubMark,
  Gear,
  Lock,
  Plus,
  Refresh,
  Search,
  Shield,
} from "@/components/icons";
import type { Server, Source } from "@/lib/data";

export function NewDeployment({
  sources,
  servers,
  account,
  stamp,
}: {
  sources: Source[];
  servers: Server[];
  account: string | null;
  stamp: string;
}) {
  const router = useRouter();
  const searchRef = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState("");
  const [source, setSource] = useState<string | null>(null);
  const [server, setServer] = useState<string | null>("falkenstein-1");

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
  const filteredSources = useMemo(
    () => (q ? sources.filter((s) => `${s.name} ${s.language}`.toLowerCase().includes(q)) : sources),
    [sources, q],
  );
  const filteredServers = useMemo(
    () => (q ? servers.filter((s) => `${s.name} ${s.provider} ${s.region}`.toLowerCase().includes(q)) : servers),
    [servers, q],
  );

  const ready = Boolean(source && server);
  const configureHref = ready
    ? `/configure?repo=${encodeURIComponent(source as string)}&server=${server}`
    : "#";

  return (
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
            onChange={(e) => setQuery(e.target.value)}
            placeholder="filter sources and servers, or jump anywhere…"
            className="flex-1 bg-transparent text-[14px] text-cream placeholder:text-muted focus:outline-none"
          />
          <kbd className="rounded-md border border-line bg-surface-2 px-2 py-0.5 text-[11px] text-muted">⌘</kbd>
          <kbd className="rounded-md border border-line bg-surface-2 px-2 py-0.5 text-[11px] text-muted">K</kbd>
        </label>
      </div>

      {/* two columns */}
      <div className="rise mt-6 grid flex-1 grid-cols-1 gap-x-16 gap-y-5 lg:grid-cols-[1.55fr_1fr]" style={{ animationDelay: "140ms" }}>
        {/* sources */}
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
              <button className="flex items-center gap-2 rounded-md border border-line bg-surface-2 px-2.5 py-1.5 text-cream">
                <GithubMark /> github <ChevronDown className="text-muted" />
              </button>
              <span className="text-faint">›</span>
              <button className="flex items-center gap-2 rounded-md border border-line bg-surface-2 px-2.5 py-1.5 text-cream">
                {account ?? "goodylili"} <ChevronDown className="text-muted" />
              </button>
            </div>
            <span className="text-muted">↳ 27 visible</span>
          </div>

          <ul className="flex-1">
            {filteredSources.map((s) => {
              const full = `${s.owner}/${s.name}`;
              const active = source === full;
              return (
                <li key={full}>
                  <button
                    onClick={() => setSource(full)}
                    className={`flex w-full items-center gap-3 px-5 py-3.5 text-left transition-colors ${
                      active ? "bg-lime/[0.08]" : "hover:bg-surface-hover"
                    }`}
                  >
                    <div className="flex-1">
                      <span className="flex items-center gap-2 text-[15px] font-medium text-cream">
                        {s.name}
                        {s.private && <Lock className="text-muted" />}
                      </span>
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
                    {active && <ArrowRight className="text-lime" />}
                  </button>
                </li>
              );
            })}
            {filteredSources.length === 0 && (
              <li className="px-5 py-8 text-center text-[13px] text-muted">no sources match “{query}”.</li>
            )}
          </ul>
        </section>

        {/* servers */}
        <section className="flex flex-col rounded-xl border border-line bg-surface">
          <div className="flex items-center justify-between border-b border-line px-5 py-4">
            <span className="flex items-center gap-2">
              <span className="label">servers</span>
              <span className="rounded border border-line px-1.5 py-0.5 text-[11px] text-muted">
                {String(filteredServers.length).padStart(2, "0")}
              </span>
            </span>
            <button className="flex items-center gap-1.5 text-[12px] text-lime transition-colors hover:text-cream">
              <Gear /> test all
            </button>
          </div>

          <ul className="flex flex-1 flex-col gap-3 p-4">
            {filteredServers.map((s) => {
              const active = server === s.id;
              return (
                <li key={s.id}>
                  <button
                    onClick={() => setServer(s.id)}
                    className={`flex w-full items-center gap-4 rounded-lg border px-4 py-3.5 text-left transition-colors ${
                      active
                        ? "border-lime/60 bg-lime/[0.05]"
                        : "border-line bg-surface-2 hover:border-line-strong"
                    }`}
                  >
                    <ServerAvatar seed={s.name} />
                    <div className="flex-1">
                      <span className="flex items-center gap-2.5 text-[15px] font-medium text-cream">
                        {s.name}
                        <Badge tone={s.status === "healthy" ? "blue" : "gray"}>
                          {s.status}
                        </Badge>
                      </span>
                      <span className="mt-1 block text-[12px] text-muted">
                        {s.provider} {s.plan} · {s.ip} · {s.region}
                      </span>
                    </div>
                    <div className="text-right">
                      <span className="block text-[15px] font-medium text-cream">
                        {s.uptimePct ?? "-"}
                      </span>
                      <span className="block text-[11px] text-muted">{s.uptimeLabel}</span>
                    </div>
                  </button>
                </li>
              );
            })}

            <li>
              <button className="flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-line-strong px-4 py-4 text-[13px] text-muted transition-colors hover:border-lime/50 hover:text-cream">
                <Plus /> add a server (ip + root password)
              </button>
            </li>
          </ul>
        </section>
      </div>

      {/* continue affordance */}
      {ready && (
        <Link
          href={configureHref}
          className="rise mt-5 flex items-center justify-between rounded-xl border border-lime/50 bg-lime/[0.06] px-5 py-4 transition-colors hover:bg-lime/[0.1]"
        >
          <span className="flex items-center gap-3 text-[14px] text-cream">
            <span className="text-lime">→ configure</span>
            {source}
            <span className="text-faint">→</span>
            {server}
          </span>
          <span className="flex items-center gap-2 text-[13px] text-lime">
            continue
            <kbd className="rounded-md border border-lime/40 px-2 py-0.5 text-[11px]">↵ enter</kbd>
          </span>
        </Link>
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
  );
}
