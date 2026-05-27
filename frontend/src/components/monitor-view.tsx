"use client";

import { useState } from "react";
import { Badge } from "@/components/badge";
import { ServerAvatar } from "@/components/server-avatar";
import { ArrowRight, ChevronRight, Refresh } from "@/components/icons";
import type { Deployment, DeployRun, RunStatus, Server } from "@/lib/data";

const runColor: Record<RunStatus, string> = {
  success: "bg-blue",
  failed: "bg-red-400",
  running: "bg-lime",
};

const statusTone = { live: "blue", idle: "gray", failing: "red" } as const;

export function MonitorView({
  deployments,
  servers,
  stamp,
}: {
  deployments: Deployment[];
  servers: Server[];
  stamp: string;
}) {
  const [open, setOpen] = useState<string>(deployments[0]?.app ?? "");
  const serverById = new Map(servers.map((s) => [s.id, s]));

  const liveCount = deployments.filter((d) => d.status === "live").length;

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
        <button className="mt-2 flex shrink-0 items-center gap-2 rounded-md border border-line px-3 py-2 text-[12px] text-lime transition-colors hover:bg-lime/10">
          <Refresh /> refresh
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
                  <span className="block text-[15px] font-medium text-cream">{d.uptimePct}</span>
                  <span className="block text-[11px] text-muted">{d.lastDeploy}</span>
                </div>
              </button>

              {isOpen && (
                <div className="border-t border-line px-5 py-4">
                  <div className="flex flex-wrap items-center gap-x-6 gap-y-2 text-[12.5px]">
                    <Metric label="cpu" value={d.metrics.cpu} />
                    <Metric label="mem" value={d.metrics.mem} />
                    <Metric label="ping" value={d.metrics.ping} />
                    <a
                      href={d.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="ml-auto flex items-center gap-1.5 text-lime transition-colors hover:text-cream"
                    >
                      open {d.url.replace("https://", "")} <ArrowRight />
                    </a>
                  </div>

                  <ul className="mt-4 divide-y divide-line border-t border-line">
                    {d.runs.map((r) => (
                      <RunRow key={r.sha} run={r} />
                    ))}
                  </ul>
                </div>
              )}
            </section>
          );
        })}
      </div>
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
