"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { Badge } from "@/components/badge";
import { ServerAvatar } from "@/components/server-avatar";
import { StreamLog } from "@/components/stream-log";
import { ArrowRight, ChevronRight, Refresh } from "@/components/icons";
import type { Deployment, DeployRun, RunStatus, Server } from "@/lib/data";
import {
  type ServerMetrics,
  fetchServerMetrics,
  fmtDisk,
  fmtLoad,
  fmtMem,
  fmtUptime,
} from "@/lib/server-metrics";

const runColor: Record<RunStatus, string> = {
  success: "bg-blue",
  failed: "bg-red-400",
  running: "bg-lime",
};

const statusTone = { live: "blue", idle: "gray", failing: "red" } as const;

// The self-hosted monitoring tools mountabo can install (hardening option ids),
// with where each lives once set up. "set up monitoring" enables this whole
// bundle on a server in one go.
const MONITORING_TOOLS = [
  { id: "netdata", label: "Netdata", access: "metrics dashboard · 127.0.0.1:19999 (ssh tunnel)" },
  { id: "uptime-kuma", label: "Uptime Kuma", access: "uptime monitor · 127.0.0.1:3001" },
  { id: "ntfy", label: "ntfy", access: "push alerts · 127.0.0.1:8080" },
  { id: "journald-persistent", label: "Persistent logs", access: "system logs kept across reboots" },
];
const MONITORING_IDS = MONITORING_TOOLS.map((t) => t.id);

export function MonitorView({
  deployments,
  servers,
  stamp,
}: {
  deployments: Deployment[];
  servers: Server[];
  stamp: string;
}) {
  const router = useRouter();
  const [refreshing, startRefresh] = useTransition();
  const [open, setOpen] = useState<string>(deployments[0]?.app ?? "");
  const [metrics, setMetrics] = useState<Record<string, ServerMetrics | null>>({});
  // Set while a monitoring-bundle install streams (the dedicated setup flow).
  const [setupServer, setSetupServer] = useState<{ id: string; name: string; set: string[] } | null>(null);
  const serverById = new Map(servers.map((s) => [s.id, s]));

  const liveCount = deployments.filter((d) => d.status === "live").length;
  const openServerId = deployments.find((d) => d.app === open)?.serverId ?? "";

  // Read the open deployment's server metrics on demand (no daemon), keyed by
  // server id so each is fetched once until a refresh clears the cache.
  useEffect(() => {
    if (!openServerId || openServerId in metrics) return;
    const ctrl = new AbortController();
    fetchServerMetrics(openServerId, ctrl.signal).then((m) =>
      setMetrics((prev) => ({ ...prev, [openServerId]: m })),
    );
    return () => ctrl.abort();
  }, [openServerId, metrics]);

  function refresh() {
    setMetrics({}); // drop cached metrics so the open server re-reads
    startRefresh(() => router.refresh()); // re-pull deployments + their runs
  }

  return (
    <main className="mx-auto flex w-full max-w-[1100px] flex-1 flex-col px-4 pb-10 pt-10 sm:px-6 sm:pt-16 lg:px-8">
      <div className="rise flex items-start justify-between gap-4 sm:gap-6">
        <div>
          <p className="label">
            live status · {stamp}
          </p>
          <h1 className="mt-6 text-4xl font-extrabold leading-[1.02] tracking-tight text-cream sm:text-5xl sm:leading-[0.98] lg:text-6xl">
            everything you ship,
            <br />
            still <span className="italic text-lime">watched.</span>
          </h1>
          <p className="mt-6 max-w-2xl text-[16px] leading-8 text-body">
            status is read from your github actions runs and on-demand pings to each server.
            mountabo checks when you open it: there is no daemon, nothing phones home.
          </p>
        </div>
        <button
          onClick={refresh}
          disabled={refreshing}
          className="mt-2 flex shrink-0 items-center gap-2 rounded-md border border-line px-3 py-2 text-[12px] text-lime transition-colors hover:bg-lime/10 disabled:opacity-60"
        >
          <Refresh className={refreshing ? "animate-spin" : ""} /> {refreshing ? "refreshing…" : "refresh"}
        </button>
      </div>

      <div className="rise mt-8 flex items-center gap-4 border-y border-line py-4 text-[13px] sm:gap-6" style={{ animationDelay: "70ms" }}>
        <Summary value={String(deployments.length)} label="apps" />
        <span className="h-8 w-px bg-line" />
        <Summary value={String(liveCount)} label="live" tone="blue" />
        <span className="h-8 w-px bg-line" />
        <Summary value={String(deployments.length - liveCount)} label="needs attention" tone={deployments.length - liveCount > 0 ? "red" : "muted"} />
      </div>

      <div className="rise mt-6 flex flex-col gap-3" style={{ animationDelay: "120ms" }}>
        {deployments.length === 0 && (
          <p className="rounded-xl border border-dashed border-line px-5 py-12 text-center text-[13px] text-muted">
            nothing deployed yet. connect a repository to a server and your live status shows up here.
          </p>
        )}
        {deployments.map((d) => {
          const server = serverById.get(d.serverId);
          const isOpen = open === d.app;
          const m = metrics[d.serverId]; // undefined = not fetched, null = unavailable
          const serverOptions = server?.options ?? [];
          const missingTools = MONITORING_IDS.filter((id) => !serverOptions.includes(id));
          const monitoringDesired = Array.from(new Set([...serverOptions, ...MONITORING_IDS]));
          return (
            <section key={d.app} className="rounded-xl border border-line bg-surface">
              <button
                onClick={() => setOpen(isOpen ? "" : d.app)}
                className="flex w-full items-center gap-4 px-5 py-4 text-left"
              >
                <ChevronRight className={`text-muted transition-transform ${isOpen ? "rotate-90" : ""}`} />
                {server && <ServerAvatar seed={server.name} />}
                <div className="min-w-0 flex-1">
                  <span className="flex items-center gap-2.5">
                    <span className="text-[15px] font-medium text-cream">{d.app}</span>
                    <Badge tone={statusTone[d.status]} dot>
                      {d.status}
                    </Badge>
                  </span>
                  <span className="mt-1 block truncate text-[12px] text-muted">
                    {d.repo} · {d.branch} · {server?.name ?? d.serverId}
                  </span>
                </div>
                <RunStrip runs={d.runs} />
                <div className="hidden text-right sm:block">
                  <span className="block text-[15px] font-medium text-cream">{d.deploys ?? 0} deploys</span>
                  <span className="block text-[11px] text-muted">last {d.lastDeploy}</span>
                </div>
              </button>

              {isOpen && (
                <div className="border-t border-line px-5 py-4">
                  <div className="flex flex-wrap items-center gap-x-6 gap-y-2 text-[12.5px]">
                    <Metric label="cpu" value={m === undefined ? "reading…" : m ? fmtLoad(m) : "n/a"} />
                    <Metric label="mem" value={m === undefined ? "reading…" : m ? fmtMem(m) : "n/a"} />
                    <Metric label="disk" value={m === undefined ? "reading…" : m ? fmtDisk(m) : "n/a"} />
                    <Metric label="uptime" value={m === undefined ? "reading…" : m ? fmtUptime(m) : "n/a"} />
                    <a
                      href={d.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="ml-auto flex items-center gap-1.5 text-lime transition-colors hover:text-cream"
                    >
                      open {d.url.replace("https://", "")} <ArrowRight />
                    </a>
                  </div>

                  {/* monitoring tools on this server */}
                  <div className="mt-4 border-t border-line pt-4">
                    <div className="mb-2 flex items-center justify-between">
                      <span className="label">monitoring tools</span>
                      {missingTools.length > 0 && (
                        <button
                          onClick={() => setSetupServer({ id: d.serverId, name: server?.name ?? d.serverId, set: monitoringDesired })}
                          className="rounded-md border border-lime/40 px-2.5 py-1 text-[11px] text-lime transition-colors hover:bg-lime/10"
                        >
                          set up monitoring ({missingTools.length})
                        </button>
                      )}
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {MONITORING_TOOLS.map((t) => {
                        const on = serverOptions.includes(t.id);
                        return (
                          <span
                            key={t.id}
                            title={t.access}
                            className={`rounded-md border px-2 py-1 text-[11px] ${
                              on ? "border-lime/40 text-lime" : "border-line text-muted"
                            }`}
                          >
                            {on ? "● " : "○ "}
                            {t.label}
                          </span>
                        );
                      })}
                    </div>
                  </div>

                  <ul className="mt-4 divide-y divide-line border-t border-line">
                    {d.runs.map((r) => (
                      <RunRow key={r.sha} run={r} />
                    ))}
                  </ul>

                  {(d.timeline?.length ?? 0) > 0 && (
                    <div className="mt-4 border-t border-line pt-4">
                      <span className="label">deploy timeline · {d.deploys ?? 0} total</span>
                      <ul className="mt-2 space-y-1.5 text-[12px]">
                        {d.timeline!.map((e, i) => (
                          <li key={i} className="flex items-center gap-3">
                            <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-blue" />
                            <span className="text-body">configured for {e.environment}</span>
                            <span className="ml-auto text-muted">{e.when}</span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>
              )}
            </section>
          );
        })}
      </div>

      {setupServer && (
        <StreamLog
          title={`setting up monitoring on ${setupServer.name}`}
          subtitle="netdata · uptime kuma · ntfy · persistent logs"
          url={`/api/servers/${setupServer.id}/options?set=${encodeURIComponent(setupServer.set.join(","))}`}
          onClose={() => setSetupServer(null)}
          onDone={(ok) => {
            if (ok) startRefresh(() => router.refresh()); // re-pull options so badges update
          }}
        />
      )}
    </main>
  );
}

function Summary({ value, label, tone = "cream" }: { value: string; label: string; tone?: "cream" | "blue" | "red" | "muted" }) {
  const color = tone === "blue" ? "text-blue" : tone === "red" ? "text-red-400" : tone === "muted" ? "text-muted" : "text-cream";
  return (
    <span className="flex items-baseline gap-2">
      <span className={`text-2xl font-bold ${color}`}>{value}</span>
      <span className="text-[12px] text-muted">{label}</span>
    </span>
  );
}

function RunStrip({ runs }: { runs: DeployRun[] }) {
  return (
    <span className="hidden items-center gap-1 md:flex" title="recent deploys">
      {runs.map((r) => (
        <span key={r.sha} className={`h-4 w-1.5 rounded-sm ${runColor[r.status]} ${r.status === "running" ? "animate-pulse" : ""}`} />
      ))}
    </span>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <span className="flex items-baseline gap-2">
      <span className="label">{label}</span>
      <span className="text-cream">{value}</span>
    </span>
  );
}

function RunRow({ run }: { run: DeployRun }) {
  const tone = run.status === "success" ? "blue" : run.status === "failed" ? "red" : "lime";
  return (
    <li className="flex items-center gap-3 py-2.5 text-[12.5px]">
      <span className={`h-1.5 w-1.5 rounded-full ${runColor[run.status]}`} />
      <code className="text-cream">{run.sha}</code>
      <span className="min-w-0 flex-1 truncate text-body">{run.message}</span>
      <Badge tone={tone}>{run.status}</Badge>
      <span className="w-16 text-right text-muted">{run.duration}</span>
      <span className="hidden w-16 text-right text-muted sm:inline">{run.when}</span>
    </li>
  );
}
