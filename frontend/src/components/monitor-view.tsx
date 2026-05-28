"use client";

import { useEffect, useRef, useState, useTransition } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Badge } from "@/components/badge";
import { ServerAvatar } from "@/components/server-avatar";
import { StreamLog } from "@/components/stream-log";
import { ConfirmAction } from "@/components/confirm-action";
import { ServerDomains, type DomainFormValue } from "@/components/server-domains";
import {
  ArrowRight,
  ChevronRight,
  CircleCheck,
  CircleDot,
  CircleX,
  ExternalLink,
  GithubMark,
  Refresh,
  Terminal,
  Trash,
} from "@/components/icons";
import type { Deployment, DeployRun, RunStatus } from "@/lib/data";
import type { Domain, ServerView } from "@/lib/servers";
import {
  type ServerMetrics,
  fetchServerMetrics,
  fmtDisk,
  fmtLoad,
  fmtMem,
  fmtUptime,
} from "@/lib/server-metrics";
import {
  type GithubConclusion,
  type GithubStatus,
  type RunJob,
  type RunSteps,
  fetchRunSteps,
  runActive,
} from "@/lib/run-steps";
import { isLogHeader, logHeaderName, splitLogTimestamp, formatLogTimestamp, fetchServerLogs } from "@/lib/server-logs";
import { classifyJobLogLine, fetchJobLogs } from "@/lib/job-logs";
import { type DomainPreview, fetchDomainPreview } from "@/lib/domain-preview";
import { type DashboardTool, installedDashboards, openDashboard } from "@/lib/dashboards";
import { type AppHealth, deleteDeployment, fetchAppHealth } from "@/lib/app-health";
import { getRepoBranches } from "@/lib/branches";
import { type UptimeKumaAdmin, fetchUptimeKumaAdmin, resetUptimeKumaAdmin } from "@/lib/uptime-kuma-admin";

const runColor: Record<RunStatus, string> = {
  success: "bg-blue",
  failed: "bg-red-400",
  running: "bg-lime",
};

const statusTone = { live: "blue", idle: "gray", failing: "red" } as const;

// How often to re-poll the GitHub run walkthrough while a run is in progress.
const STEP_POLL_MS = 4000;

export function MonitorView({
  deployments,
  servers,
  stamp,
}: {
  deployments: Deployment[];
  servers: ServerView[];
  stamp: string;
}) {
  const router = useRouter();
  // After a successful deploy the configure page sends the operator here with
  // ?app=<name>, so open that deployment; otherwise open the first one.
  const appParam = useSearchParams().get("app");
  const [open, setOpen] = useState<string>(
    (appParam && deployments.some((d) => d.app === appParam) ? appParam : deployments[0]?.app) ?? "",
  );
  const [refreshing, startRefresh] = useTransition();
  const [metrics, setMetrics] = useState<Record<string, ServerMetrics | null>>({});
  const [serverList, setServerList] = useState<ServerView[]>(servers);
  const serverById = new Map(serverList.map((s) => [s.id, s]));

  // App health per deployment (keyed by app): undefined = not probed yet, null =
  // could not reach the backend (unknown), otherwise the probe result.
  const [health, setHealth] = useState<Record<string, AppHealth | null>>({});
  // The delete confirmation gate (the open deployment to forget), and the set of
  // apps already dropped locally so their cards disappear immediately on delete.
  const [confirmDelete, setConfirmDelete] = useState<Deployment | null>(null);
  const [deletedApps, setDeletedApps] = useState<Set<string>>(new Set());
  const visibleDeployments = deployments.filter((d) => !deletedApps.has(d.app));

  // Probe the open deployment's health on demand (no daemon), keyed by app so it
  // is fetched once per open until a refresh clears the cache. setState only runs
  // in the resolved callback, never synchronously in the effect body.
  useEffect(() => {
    if (!open || open in health) return;
    const ctrl = new AbortController();
    fetchAppHealth(open, ctrl.signal).then((h) => setHealth((prev) => ({ ...prev, [open]: h })));
    return () => ctrl.abort();
  }, [open, health]);

  // confirmDeployDelete forgets a deployment's tracking, drops its card locally,
  // then refreshes to re-pull authoritative state. Runs from an event handler.
  async function confirmDeployDelete(app: string) {
    setConfirmDelete(null);
    const ok = await deleteDeployment(app);
    if (!ok) return;
    setDeletedApps((prev) => new Set(prev).add(app));
    startRefresh(() => router.refresh());
  }

  // Live streams (add domain / remove domain) for the open card. Each only
  // opens after its ConfirmAction gate is confirmed.
  const [domainTarget, setDomainTarget] = useState<
    | { server: ServerView; mode: "add"; value: DomainFormValue }
    | { server: ServerView; mode: "remove"; host: string }
    | null
  >(null);

  // Confirmation gates. Nothing touches a server until one is confirmed.
  const [confirmDomain, setConfirmDomain] = useState<
    | { server: ServerView; mode: "add"; value: DomainFormValue }
    | { server: ServerView; mode: "remove"; host: string }
    | null
  >(null);
  // The add-domain preview (nginx config + script), keyed by the host it was
  // fetched for. Keying by host means a stale preview from a previous host is
  // never shown and the effect never resets state synchronously; data null means
  // the fetch was attempted but returned nothing.
  const [previewState, setPreviewState] = useState<{ host: string; data: DomainPreview | null } | null>(null);

  const liveCount = visibleDeployments.filter((d) => d.status === "live").length;
  const openDeployment = visibleDeployments.find((d) => d.app === open);
  const openServerId = openDeployment?.serverId ?? "";

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

  // When the add-domain gate opens, fetch the exact nginx config + script the
  // backend would write, so the operator sees precisely what runs before it
  // does. State is only set in the resolved callback (never synchronously in the
  // effect body); staleness is handled by keying the result to the host.
  useEffect(() => {
    if (!confirmDomain || confirmDomain.mode !== "add") return;
    const host = confirmDomain.value.host;
    const ctrl = new AbortController();
    fetchDomainPreview(confirmDomain.value, ctrl.signal)
      .then((res) => {
        if (!ctrl.signal.aborted) setPreviewState({ host, data: "error" in res ? null : res });
      })
      .catch(() => {
        if (!ctrl.signal.aborted) setPreviewState({ host, data: null });
      });
    return () => ctrl.abort();
  }, [confirmDomain]);

  // Derived preview for the open add-domain gate: the data when it is for the
  // current host, and a loading flag while this host's fetch is still pending.
  const addHost = confirmDomain?.mode === "add" ? confirmDomain.value.host : null;
  const domainPreview = addHost !== null && previewState?.host === addHost ? previewState.data : null;
  const domainPreviewLoading = addHost !== null && previewState?.host !== addHost;

  function refresh() {
    setMetrics({}); // drop cached metrics so the open server re-reads
    setHealth({}); // drop cached health so the open app is re-probed
    startRefresh(() => router.refresh()); // re-pull deployments + their runs
  }

  // After a successful domain change, reflect it locally so the panels stay
  // current without a refetch, then refresh to re-pull authoritative state.
  function recordDomainAdd(serverId: string, value: DomainFormValue) {
    const entry: Domain = {
      host: value.host,
      aliases: value.aliases,
      upstream: value.upstream,
      email: value.email || undefined,
      createdAt: new Date().toISOString(),
    };
    setServerList((list) =>
      list.map((s) =>
        s.id === serverId
          ? { ...s, domains: [...(s.domains ?? []).filter((d) => d.host !== entry.host), entry] }
          : s,
      ),
    );
  }
  function recordDomainRemove(serverId: string, host: string) {
    setServerList((list) =>
      list.map((s) =>
        s.id === serverId ? { ...s, domains: (s.domains ?? []).filter((d) => d.host !== host) } : s,
      ),
    );
  }

  return (
    <main className="mx-auto flex w-full max-w-[1100px] flex-1 flex-col px-4 pb-16 pt-10 sm:px-6 sm:pt-16 lg:px-8">
      <div className="rise flex items-start justify-between gap-4 sm:gap-6">
        <div>
          <p className="label">deployments · {stamp}</p>
          <h1 className="mt-6 text-4xl font-extrabold leading-[1.02] tracking-tight text-cream sm:text-5xl sm:leading-[0.98] lg:text-6xl">
            every deployment,
            <br />
            and how it is <span className="italic text-lime">running.</span>
          </h1>
          <p className="mt-6 max-w-2xl text-[16px] leading-8 text-body">
            open a deployment to walk its github actions run, read its container logs, watch host metrics,
            open its monitoring dashboards, and manage its domains. mountabo reads it all when you open it:
            there is no daemon, nothing phones home.
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

      <div
        className="rise mt-10 flex items-center gap-6 border-y border-line py-6 text-[13px] sm:gap-10"
        style={{ animationDelay: "70ms" }}
      >
        <Summary value={String(visibleDeployments.length)} label="apps" />
        <span className="h-10 w-px bg-line" />
        <Summary value={String(liveCount)} label="live" tone="blue" />
        <span className="h-10 w-px bg-line" />
        <Summary
          value={String(visibleDeployments.length - liveCount)}
          label="needs attention"
          tone={visibleDeployments.length - liveCount > 0 ? "red" : "muted"}
        />
      </div>

      <div className="rise mt-8 flex flex-col gap-5" style={{ animationDelay: "120ms" }}>
        {visibleDeployments.length === 0 && (
          <p className="rounded-2xl border border-dashed border-line px-6 py-16 text-center text-[14px] text-muted">
            nothing deployed yet. connect a repository to a server and your live status shows up here.
          </p>
        )}
        {visibleDeployments.map((d) => {
          const server = serverById.get(d.serverId);
          const isOpen = open === d.app;
          const m = metrics[d.serverId]; // undefined = not fetched, null = unavailable
          const latest = d.runs[0];
          return (
            <section key={d.app} className="relative overflow-hidden rounded-2xl border border-line bg-surface">
              <button
                onClick={() => setOpen(isOpen ? "" : d.app)}
                className="flex w-full items-center gap-4 px-6 py-5 pr-16 text-left"
                aria-expanded={isOpen}
              >
                <ChevronRight className={`text-muted transition-transform ${isOpen ? "rotate-90" : ""}`} />
                {server && <ServerAvatar seed={server.name} />}
                <div className="min-w-0 flex-1">
                  <span className="flex flex-wrap items-center gap-3">
                    <span className="text-[18px] font-semibold text-cream">{d.app}</span>
                    <Badge tone={statusTone[d.status]} dot>
                      {d.status}
                    </Badge>
                    <HealthPill health={health[d.app]} />
                    {latest && <LatestRunPill status={latest.status} />}
                  </span>
                  <span className="mt-1.5 block truncate text-[13px] text-muted">
                    {d.repo} · {d.branch} · {server?.name ?? d.serverId}
                  </span>
                </div>
                <RunStrip runs={d.runs} />
                <div className="hidden text-right sm:block">
                  <span className="block text-[18px] font-semibold text-cream">{d.deploys ?? 0}</span>
                  <span className="block text-[11px] text-muted">deploys · last {d.lastDeploy}</span>
                </div>
              </button>

              {isOpen && (
                <ExpandedCard
                  deployment={d}
                  server={server}
                  metrics={m}
                  health={health[d.app]}
                  onAddDomain={(server, value) =>
                    setConfirmDomain({ server, mode: "add", value })
                  }
                  onRemoveDomain={(server, host) =>
                    setConfirmDomain({ server, mode: "remove", host })
                  }
                  onDelete={() => setConfirmDelete(d)}
                />
              )}
            </section>
          );
        })}
      </div>

      {/* Live streams: each opens only after its gate is confirmed. */}
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
            if (domainTarget.mode === "add")
              recordDomainAdd(domainTarget.server.id, domainTarget.value);
            else recordDomainRemove(domainTarget.server.id, domainTarget.host);
            startRefresh(() => router.refresh());
          }}
          onClose={() => setDomainTarget(null)}
        />
      )}

      {/* Confirmation gates. */}
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

      {/* Delete a deployment: type-to-confirm, honest about what it does and
          does not do. It only removes mountabo's local tracking. */}
      {confirmDelete && (
        <ConfirmAction
          title={`delete ${confirmDelete.app}`}
          subtitle="tear this deployment down"
          destructive
          requireTyping={confirmDelete.app}
          summary={
            <>
              this tears down <span className="text-cream">{confirmDelete.app}</span>. mountabo stops and
              removes its running container on{" "}
              <span className="text-cream">{serverById.get(confirmDelete.serverId)?.name ?? confirmDelete.serverId}</span>,
              deletes the deploy workflow from{" "}
              <span className="text-cream">{confirmDelete.repo}</span> so a future push no longer deploys it,
              and forgets the deployment record and its history. this cannot be undone. type the app name to
              confirm.
            </>
          }
          stepsLabel="what deleting does"
          steps={[
            `stop and remove the running container for ${confirmDelete.app} on ${serverById.get(confirmDelete.serverId)?.name ?? confirmDelete.serverId}`,
            `delete the deploy workflow and deploy.sh from ${confirmDelete.repo} so it no longer deploys`,
            "remove the deployment record and its append-only history from mountabo",
          ]}
          confirmLabel="delete deployment"
          onConfirm={() => void confirmDeployDelete(confirmDelete.app)}
          onCancel={() => setConfirmDelete(null)}
        />
      )}
    </main>
  );
}

// HealthPill is the prominent up/down indicator on a deployment card, from the
// SSH-based app probe. undefined while it is still being read, null when the
// probe could not reach the backend (shown as "health unknown"). Otherwise it
// shows healthy (any HTTP response) or unhealthy (no response), with the HTTP
// status when there is one.
function HealthPill({ health }: { health: AppHealth | null | undefined }) {
  if (health === undefined) {
    return <Badge tone="gray">checking health</Badge>;
  }
  if (health === null) {
    return <Badge tone="gray">health unknown</Badge>;
  }
  if (health.reachable) {
    return (
      <Badge tone="blue" dot>
        healthy{health.status ? ` · ${health.status}` : ""}
      </Badge>
    );
  }
  return (
    <Badge tone="red" dot>
      unhealthy{health.status ? ` · ${health.status}` : ""}
    </Badge>
  );
}

// HealthBanner is the prominent app-health read inside the open card: a clear
// healthy/unhealthy state, what was probed, and an honest "unknown" when the
// probe could not reach the backend. It is the SSH up/down indicator; the
// embedded Uptime Kuma dashboard is the detailed view.
function HealthBanner({ health }: { health: AppHealth | null | undefined }) {
  if (health === undefined) {
    return (
      <div className="mt-6 flex items-center gap-3 rounded-xl border border-line bg-surface/40 px-5 py-4">
        <CircleDot className="animate-pulse text-muted" />
        <span className="text-[14px] text-muted">checking whether the app is responding, over ssh…</span>
      </div>
    );
  }
  if (health === null) {
    return (
      <div className="mt-6 flex items-center gap-3 rounded-xl border border-line bg-surface/40 px-5 py-4">
        <CircleDot className="text-muted" />
        <span className="text-[14px] text-muted">
          app health is unknown. mountabo could not probe the app (the server may not be set up, or is
          unreachable).
        </span>
      </div>
    );
  }
  if (health.reachable) {
    return (
      <div className="mt-6 flex items-center gap-3 rounded-xl border border-blue/30 bg-blue/[0.06] px-5 py-4">
        <CircleCheck className="text-blue" />
        <span className="text-[14px] text-cream">
          app is healthy{health.status ? ` (http ${health.status})` : ""}
          {health.target && <span className="text-muted"> · {health.target}</span>}
        </span>
      </div>
    );
  }
  return (
    <div className="mt-6 flex items-center gap-3 rounded-xl border border-red-500/30 bg-red-500/[0.06] px-5 py-4">
      <CircleX className="text-red-400" />
      <span className="text-[14px] text-cream">
        app is not responding{health.status ? ` (http ${health.status})` : ""}
        {health.detail && <span className="text-muted"> · {health.detail}</span>}
      </span>
    </div>
  );
}

// ExpandedCard is the open deployment's full body: deploy status + live link,
// host metrics, the live GitHub run walkthrough, the runs list, a logs viewer,
// per-tool monitoring, custom domains, and the deploy timeline.
function ExpandedCard({
  deployment: d,
  server,
  metrics: m,
  health,
  onAddDomain,
  onRemoveDomain,
  onDelete,
}: {
  deployment: Deployment;
  server?: ServerView;
  metrics: ServerMetrics | null | undefined;
  health: AppHealth | null | undefined;
  onAddDomain: (server: ServerView, value: DomainFormValue) => void;
  onRemoveDomain: (server: ServerView, host: string) => void;
  onDelete: () => void;
}) {
  const latest = d.runs[0];
  const [owner, repo] = splitRepo(d.repo);
  const live = d.liveUrl ? d.liveUrl.replace(/^https?:\/\//, "") : "";
  const dashboards = installedDashboards(server?.options);
  const router = useRouter();
  // New-environment form state: the branch the operator wants to deploy as a
  // sibling environment. Submitting sends them to the configure page prefilled
  // with this repo + server + the new branch, where they fill in the
  // environment's name and variables and click deploy, exactly like the first
  // environment was created.
  const [newBranch, setNewBranch] = useState("");
  // Branches fetched from GitHub for this repo, so the picker is a real
  // dropdown of the operator's branches instead of a free-text field. null
  // means "not loaded yet"; [] means "loaded but no branches came back" (in
  // which case the form falls back to a text input).
  const [branches, setBranches] = useState<string[] | null>(null);

  // Fetch branches once the environments tab is first opened. setState only
  // runs once the fetch resolves (never synchronously in the effect body),
  // matching the codebase's react-hooks/set-state-in-effect rule.

  // Custom domains are scoped to this environment: only the domains pointing
  // at this deployment's port show on the card, and the add form pre-fills
  // that port so adding one is a single field. Pre-computed so the tab bar
  // can show the count and the panel can render without re-deriving it.
  const envPort = d.port > 0 ? String(d.port) : undefined;
  const envDomains = envPort
    ? (server?.domains ?? []).filter((dm) => dm.upstream === envPort)
    : (server?.domains ?? []);

  // Tab definitions for the segmented control at the top of the card. Each
  // tab is one panel; only the active one renders below, which keeps the
  // card short and scannable. Optional tabs (dashboards, domains, timeline)
  // appear only when they have something to show, matching the previous
  // collapsible-section visibility rules.
  type TabKey =
    | "environments"
    | "walkthrough"
    | "runs"
    | "logs"
    | "dashboards"
    | "domains"
    | "timeline";

  const timelineCount = d.timeline?.length ?? 0;
  const tabs: { key: TabKey; label: string }[] = [
    { key: "environments", label: "environments" },
    { key: "walkthrough", label: "deploy walkthrough" },
    { key: "runs", label: `recent runs (${d.runs.length})` },
    { key: "logs", label: "container logs" },
    ...(server && dashboards.length > 0
      ? [{ key: "dashboards" as const, label: `monitoring (${dashboards.length})` }]
      : []),
    ...(server
      ? [{ key: "domains" as const, label: `custom domains (${envDomains.length})` }]
      : []),
    ...(timelineCount > 0
      ? [{ key: "timeline" as const, label: `deploy timeline (${d.deploys ?? 0})` }]
      : []),
  ];

  // Default to "environments" so the operator lands on the per-environment
  // controls (current branch + add another) which is the highest-value view
  // when opening the card. Other tabs are one click away.
  const [tab, setTab] = useState<TabKey>("environments");

  // Pull GitHub branches lazily, once the environments tab is in view, so the
  // picker shows real branches the operator can pick from. setState is only
  // called inside the resolved callback, never synchronously in the effect.
  useEffect(() => {
    if (tab !== "environments" || branches !== null) return;
    const ctrl = new AbortController();
    getRepoBranches(owner, repo, ctrl.signal).then((list) => {
      if (!ctrl.signal.aborted) setBranches(list);
    });
    return () => ctrl.abort();
  }, [tab, branches, owner, repo]);

  // Branches the operator can pick from: everything except the current one
  // (re-deploying the same branch creates a duplicate environment, not a new
  // one). When no branches loaded yet we fall back to a text input.
  const otherBranches = (branches ?? []).filter((b) => b !== d.branch);

  return (
    <div className="border-t border-line px-6 py-8 sm:px-8">
      {/* deploy status + live link: the always-visible header of the open card */}
      <div className="flex flex-col gap-5 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-4">
          <DeployStatusBadge status={latest?.status} />
          <div>
            <p className="text-[17px] font-medium text-cream">
              {latest
                ? latest.status === "success"
                  ? "latest deployment succeeded"
                  : latest.status === "failed"
                    ? "latest deployment failed"
                    : "deployment is running"
                : "no deployment runs yet"}
            </p>
            <p className="mt-1 text-[14px] text-muted">
              {latest ? `${latest.message} · ${latest.when}` : "trigger a deploy to see status here"}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2.5">
          {d.liveUrl && (
            <a
              href={d.liveUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="cta-glow flex items-center gap-2 rounded-lg bg-lime-fill px-4 py-2.5 text-[14px] font-bold text-black transition-transform hover:-translate-y-0.5"
            >
              open {live} <ArrowRight />
            </a>
          )}
          {d.workflowUrl && (
            <a
              href={d.workflowUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 rounded-lg border border-line px-4 py-2.5 text-[13.5px] text-muted transition-colors hover:border-line-strong hover:text-cream"
            >
              <GithubMark /> actions workflow <ExternalLink />
            </a>
          )}
        </div>
      </div>

      {/* app health: a prominent up/down read, probed from the server over SSH.
          The embedded Uptime Kuma dashboard below is the detailed view. */}
      <HealthBanner health={health} />

      <div className="mt-8 space-y-5">
        {/* host metrics: small enough to stay above the tab bar so cpu/memory
            /disk/uptime are visible no matter which tab is active. */}
        <div className="grid grid-cols-2 gap-px overflow-hidden rounded-xl border border-line bg-line sm:grid-cols-4">
          <BigMetric label="cpu" value={m === undefined ? "reading…" : m ? fmtLoad(m) : "n/a"} />
          <BigMetric label="memory" value={m === undefined ? "reading…" : m ? fmtMem(m) : "n/a"} />
          <BigMetric label="disk" value={m === undefined ? "reading…" : m ? fmtDisk(m) : "n/a"} />
          <BigMetric label="uptime" value={m === undefined ? "reading…" : m ? fmtUptime(m) : "n/a"} />
        </div>

        {/* tab bar: same segmented-control pattern as the home page's
            VISIBILITY/CONTAINER filter chips. An uppercase label anchors the
            row, the chips wrap when there isn't room, and only the active
            panel renders below so the card stays short. */}
        <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
          <span className="text-[12px] font-medium uppercase tracking-wide text-muted">section</span>
          <span
            role="tablist"
            aria-label="deployment sections"
            className="inline-flex max-w-full flex-wrap items-center gap-1 rounded-lg border border-line bg-surface p-1"
          >
            {tabs.map((t) => {
              const active = tab === t.key;
              return (
                <button
                  key={t.key}
                  role="tab"
                  aria-selected={active}
                  onClick={() => setTab(t.key)}
                  className={`rounded-md px-3.5 py-1.5 text-[13px] font-medium transition-colors ${
                    active
                      ? "bg-lime/15 text-lime"
                      : "text-muted hover:bg-surface-2 hover:text-cream"
                  }`}
                >
                  {t.label}
                </button>
              );
            })}
          </span>
        </div>

        {/* environments: this deployment is one (branch, environment) pair; the
            operator can create another one for the same repo from here without
            going back to the home picker. */}
        {tab === "environments" && (
          <div className="space-y-4">
            <div className="flex items-center justify-between rounded-xl border border-line bg-surface px-4 py-3 text-[13.5px]">
              <span className="flex items-center gap-3">
                <span className="h-1.5 w-1.5 rounded-full bg-lime" />
                <span className="text-cream">{d.branch}</span>
                <span className="text-muted">on branch {d.branch}</span>
              </span>
              <span className="text-[12px] text-muted">current</span>
            </div>

            <form
              onSubmit={(e) => {
                e.preventDefault();
                const b = newBranch.trim();
                if (!b) return;
                router.push(
                  `/configure?repo=${encodeURIComponent(d.repo)}&branch=${encodeURIComponent(b)}&server=${encodeURIComponent(d.serverId)}`,
                );
              }}
              className="rounded-xl border border-dashed border-line bg-surface/40 p-4"
            >
              <p className="text-[12px] uppercase tracking-wide text-muted">add another environment</p>
              <p className="mt-1 text-[13px] leading-6 text-body">
                pick a branch from this repo. on the next page you name the environment, add its
                variables, and set up its custom domain, then click deploy to ship it.
              </p>
              <div className="mt-3 flex flex-col gap-2 sm:flex-row">
                {otherBranches.length > 0 ? (
                  <select
                    value={newBranch}
                    onChange={(e) => setNewBranch(e.target.value)}
                    className="min-w-0 flex-1 rounded-md border border-line bg-bg px-3 py-2 font-mono text-[13px] text-cream focus:border-line-strong focus:outline-none"
                  >
                    <option value="" className="text-faint">
                      pick a branch from github,
                    </option>
                    {otherBranches.map((b) => (
                      <option key={b} value={b}>
                        {b}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    value={newBranch}
                    onChange={(e) => setNewBranch(e.target.value)}
                    placeholder={
                      branches === null
                        ? "reading branches from github,"
                        : "branch name, for example production or staging"
                    }
                    spellCheck={false}
                    autoCapitalize="off"
                    autoCorrect="off"
                    className="min-w-0 flex-1 rounded-md border border-line bg-bg px-3 py-2 font-mono text-[13px] text-cream placeholder:text-faint focus:border-line-strong focus:outline-none"
                  />
                )}
                <button
                  type="submit"
                  disabled={newBranch.trim() === ""}
                  className="shrink-0 rounded-md border border-lime/50 bg-lime/[0.08] px-4 py-2 text-[13px] font-medium text-lime transition-colors hover:bg-lime/[0.16] disabled:cursor-not-allowed disabled:opacity-40"
                >
                  configure environment and domains
                </button>
              </div>
            </form>
          </div>
        )}

        {/* step-by-step deploy walkthrough from GitHub Actions */}
        {tab === "walkthrough" && (
          <RunWalkthrough owner={owner} repo={repo} branch={d.branch} latestStatus={latest?.status} />
        )}

        {/* runs list with clickable GitHub links */}
        {tab === "runs" &&
          (d.runs.length === 0 ? (
            <p className="text-[14px] text-muted">no runs recorded yet.</p>
          ) : (
            <ul className="divide-y divide-line overflow-hidden rounded-xl border border-line">
              {d.runs.map((r) => (
                <RunRow key={r.sha} run={r} />
              ))}
            </ul>
          ))}

        {/* logs viewer */}
        {tab === "logs" && (
          <LogsViewer serverId={d.serverId} ready={Boolean(server)} active />
        )}

        {/* monitoring dashboards reached through the ssh tunnel */}
        {tab === "dashboards" && server && dashboards.length > 0 && (
          <DashboardsPanel serverId={server.id} dashboards={dashboards} active />
        )}

        {/* custom domains, scoped to this environment: only domains pointing at
            this deployment's port are listed and the add form defaults to it,
            so each environment manages its own domains from its own card. */}
        {tab === "domains" && server && (
          <div className="overflow-hidden rounded-xl border border-line bg-surface">
            <ServerDomains
              key={`dom:${envPort ?? "all"}:${envDomains.map((dm) => dm.host).join(",")}`}
              server={server}
              onAdd={(value) => onAddDomain(server, value)}
              onRemove={(host) => onRemoveDomain(server, host)}
              filterUpstream={envPort}
              defaultPort={envPort}
            />
          </div>
        )}

        {/* deploy timeline */}
        {tab === "timeline" && timelineCount > 0 && (
          <ul className="space-y-3 text-[14px]">
            {(d.timeline ?? []).map((e, i) => (
              <li key={i} className="flex items-center gap-3">
                <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-blue" />
                <span className="text-body">configured for {e.environment}</span>
                <span className="ml-auto text-muted">{e.when}</span>
              </li>
            ))}
          </ul>
        )}

        {/* danger zone: deleting tears the deployment down, so it gets its own
            bordered section at the bottom of the card rather than a small icon. */}
        <div className="mt-2 rounded-xl border border-red-400/30 bg-red-400/[0.03] p-5">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p className="text-[15px] font-medium text-cream">delete this deployment</p>
              <p className="mt-1 max-w-xl text-[13px] leading-6 text-muted">
                stops and removes the running container, disables the deploy workflow on the repo,
                and forgets the record. this cannot be undone.
              </p>
            </div>
            <button
              onClick={onDelete}
              className="flex shrink-0 items-center gap-2 rounded-md border border-red-400/50 px-4 py-2 text-[13px] font-medium text-red-300 transition-colors hover:bg-red-400/10"
            >
              <Trash /> delete deployment
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

// DashboardsPanel shows each installed monitoring tool's dashboard, reached
// through an SSH local port-forward tunnel the backend opens on demand (the tool
// binds to the server's loopback, so it is never exposed publicly). When the
// section is open it asks the backend to open the tunnel, gets back a local
// http://127.0.0.1:<port>/ URL, and loads it straight in an iframe: because the
// tunnel carries raw TCP, the tool is served at root and its websockets connect
// directly. It also offers an "open in a tab" link to that same local URL.
function DashboardsPanel({
  serverId,
  dashboards,
  active,
}: {
  serverId: string;
  dashboards: DashboardTool[];
  active: boolean;
}) {
  return (
    <div className="space-y-5">
      {dashboards.map((tool) => (
        <DashboardCard key={tool.id} serverId={serverId} tool={tool} active={active} />
      ))}
    </div>
  );
}

// DashboardCard opens (and reuses) one tool's tunnel when its section is active,
// then loads the returned local URL in an iframe. It surfaces an honest opening
// state and an error state rather than a silently broken frame.
function DashboardCard({
  serverId,
  tool,
  active,
}: {
  serverId: string;
  tool: DashboardTool;
  active: boolean;
}) {
  // url is the local tunnel address once open; errored is set when it could not
  // be opened. "opening" is derived (active, no url, not errored), so the effect
  // never sets state synchronously. nonce re-triggers the open on retry.
  const [url, setUrl] = useState<string | null>(null);
  const [errored, setErrored] = useState(false);
  const [nonce, setNonce] = useState(0);
  // Uptime Kuma has no public HTTP setup endpoint, so the operator can never
  // guess the admin password. mountabo offers to generate one and seed it into
  // UK's SQLite from inside the container; the result is shown above the
  // iframe so the operator can copy it into the login form below. null = not
  // loaded; { hasAdmin:false } = none stored; { hasAdmin:true, ... } = stored.
  const isKuma = tool.id === "uptime-kuma";
  const [admin, setAdmin] = useState<UptimeKumaAdmin | null>(null);
  const [resetting, setResetting] = useState(false);
  const [showPassword, setShowPassword] = useState(false);

  // Read the stored credentials the first time the section becomes active. The
  // setState only runs once the fetch resolves, never synchronously here.
  useEffect(() => {
    if (!isKuma || !active || admin !== null) return;
    const ctrl = new AbortController();
    fetchUptimeKumaAdmin(serverId, ctrl.signal).then((res) => {
      if (!ctrl.signal.aborted) setAdmin(res);
    });
    return () => ctrl.abort();
  }, [isKuma, active, serverId, admin]);

  async function setUpAdmin() {
    if (resetting) return;
    setResetting(true);
    const next = await resetUptimeKumaAdmin(serverId);
    if (next) {
      setAdmin(next);
      setShowPassword(true);
    }
    setResetting(false);
  }

  // Open the tunnel the first time the section becomes active. The backend
  // reuses an existing tunnel for the same (server, tool) pair, so re-opening is
  // cheap. setState only runs once the fetch resolves, never synchronously.
  useEffect(() => {
    if (!active || url !== null || errored) return;
    const ctrl = new AbortController();
    openDashboard(serverId, tool.id, ctrl.signal).then((res) => {
      if (ctrl.signal.aborted) return;
      if (res) setUrl(res.url);
      else setErrored(true);
    });
    return () => ctrl.abort();
  }, [active, serverId, tool.id, url, errored, nonce]);

  function retry() {
    setUrl(null);
    setErrored(false);
    setNonce((n) => n + 1);
  }

  return (
    <div className="overflow-hidden rounded-xl border border-line bg-surface">
      <div className="flex items-center justify-between gap-3 border-b border-line px-5 py-3.5">
        <span className="flex items-center gap-2.5">
          <span className="h-2 w-2 rounded-full bg-lime" />
          <span className="text-[15px] font-medium text-cream">{tool.label}</span>
        </span>
      </div>
      {isKuma && (
        <div className="border-b border-line bg-surface/60 px-5 py-3.5">
          {admin === null ? (
            <p className="text-[12.5px] text-muted">reading admin credentials…</p>
          ) : admin.hasAdmin ? (
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="min-w-0 text-[12.5px] leading-6 text-body">
                <p className="text-muted">
                  log in with these credentials. mountabo generated them and seeded them into uptime kuma.
                </p>
                <p className="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 font-mono text-[13px]">
                  <span>
                    <span className="text-muted">username</span>{" "}
                    <span className="select-all text-cream">{admin.username}</span>
                  </span>
                  <span>
                    <span className="text-muted">password</span>{" "}
                    <span className="select-all text-cream">
                      {showPassword ? admin.password : "•".repeat(admin.password.length)}
                    </span>
                  </span>
                  <button
                    onClick={() => setShowPassword((v) => !v)}
                    className="rounded-md border border-line px-2 py-0.5 text-[11.5px] text-muted transition-colors hover:border-line-strong hover:text-cream"
                  >
                    {showPassword ? "hide" : "show"}
                  </button>
                </p>
              </div>
              <button
                onClick={() => void setUpAdmin()}
                disabled={resetting}
                className="shrink-0 rounded-md border border-line px-3 py-1.5 text-[12px] text-muted transition-colors hover:border-line-strong hover:text-cream disabled:opacity-50"
              >
                {resetting ? "regenerating…" : "regenerate"}
              </button>
            </div>
          ) : (
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <p className="text-[12.5px] leading-6 text-body">
                uptime kuma is asking for an admin to log in, but it has no public setup api. click set up
                admin and mountabo will generate a password, seed it directly into the container, and show
                it once.
              </p>
              <button
                onClick={() => void setUpAdmin()}
                disabled={resetting}
                className="shrink-0 rounded-md border border-lime/50 bg-lime/[0.08] px-4 py-2 text-[12.5px] font-medium text-lime transition-colors hover:bg-lime/[0.16] disabled:opacity-50"
              >
                {resetting ? "setting up…" : "set up admin"}
              </button>
            </div>
          )}
        </div>
      )}
      {url ? (
        <div className="relative">
          {/* The backend's reverse proxy strips X-Frame-Options and CSP from the
              dashboard's responses, so the iframe loads even though it is a
              different origin from the app. A "pop out" link is kept beside it
              for full-screen use. */}
          <iframe
            src={url}
            title={tool.label}
            className="block h-[640px] w-full border-0 bg-black"
            // Allow same-origin so the dashboard's cookies/auth keep working,
            // and allow scripts + forms so its UI is interactive.
            sandbox="allow-same-origin allow-scripts allow-forms allow-popups allow-modals"
          />
          <a
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            className="absolute right-3 top-3 flex items-center gap-1.5 rounded-md border border-line bg-surface/95 px-2.5 py-1.5 text-[12px] text-muted backdrop-blur transition-colors hover:border-line-strong hover:text-cream"
          >
            open in a tab <ExternalLink />
          </a>
        </div>
      ) : errored ? (
        <div className="flex h-[200px] flex-col items-center justify-center gap-3 bg-black/40 px-5 text-center">
          <p className="text-[14px] leading-7 text-body">
            could not open the {tool.label} tunnel over ssh. the server may be unreachable, or the tool may
            not be running.
          </p>
          <button
            onClick={retry}
            className="flex items-center gap-1.5 rounded-md border border-line px-3 py-1.5 text-[13px] text-lime transition-colors hover:bg-lime/10"
          >
            <Refresh /> try again
          </button>
        </div>
      ) : (
        <div className="flex h-[200px] items-center justify-center bg-black/40">
          <p className="text-[13px] text-muted">opening the dashboard through the ssh tunnel…</p>
        </div>
      )}
    </div>
  );
}

// RunWalkthrough fetches the open deployment's latest GitHub Actions run and
// renders its jobs and steps with live status, polling while the run is in
// progress and stopping once it completes.
function RunWalkthrough({
  owner,
  repo,
  branch,
  latestStatus,
}: {
  owner: string;
  repo: string;
  branch: string;
  latestStatus?: RunStatus;
}) {
  const [steps, setSteps] = useState<RunSteps | null>(null);
  // null = not read yet; the effect resolves it on mount and on each poll.
  const enabled = Boolean(owner && repo && branch);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    async function tick() {
      const ctrl = new AbortController();
      const res = await fetchRunSteps(owner, repo, branch, ctrl.signal);
      if (cancelled) return;
      setSteps(res);
      // Keep polling only while the run is still going; stop once it completes.
      if (runActive(res)) timer = setTimeout(tick, STEP_POLL_MS);
    }

    void tick();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
    // latestStatus is included so a refresh that flips the run back to running
    // (a new deploy) restarts polling.
  }, [enabled, owner, repo, branch, latestStatus]);

  if (enabled && steps === null) {
    return <p className="mt-4 text-[13px] text-muted">reading the latest workflow run…</p>;
  }
  if (!steps || (!steps.runUrl && steps.jobs.length === 0)) {
    return (
      <p className="mt-4 rounded-xl border border-dashed border-line px-5 py-8 text-center text-[13px] text-muted">
        no workflow run found for {branch} yet. push to {branch} or trigger the deploy workflow and the
        steps show up here.
      </p>
    );
  }

  const active = runActive(steps);
  return (
    <div className="mt-4 space-y-4">
      <div className="flex items-center gap-3 text-[12.5px]">
        <span className={`flex items-center gap-2 ${active ? "text-lime" : "text-muted"}`}>
          <span className={`h-1.5 w-1.5 rounded-full ${active ? "animate-pulse bg-lime" : "bg-muted"}`} />
          {active ? "run in progress, refreshing every few seconds" : "run finished"}
        </span>
        {steps.runUrl && (
          <a
            href={steps.runUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="ml-auto flex items-center gap-1.5 text-muted transition-colors hover:text-cream"
          >
            <GithubMark /> view on github <ExternalLink />
          </a>
        )}
      </div>

      {steps.jobs.length === 0 ? (
        <p className="text-[13px] text-muted">the run has no jobs reported yet.</p>
      ) : (
        <div className="space-y-4">
          {steps.jobs.map((job, ji) => (
            <JobPanel key={job.jobId || ji} owner={owner} repo={repo} job={job} />
          ))}
        </div>
      )}
    </div>
  );
}

// JobPanel renders one job of the run: its steps with live status, and an
// expandable log of what the job printed so the operator can read what worked
// and what failed without leaving the page. A failed job opens by default so the
// error is visible immediately. The log loads lazily the first time the panel
// opens (and only setting state inside the resolved fetch, never synchronously
// in the effect body).
function JobPanel({ owner, repo, job }: { owner: string; repo: string; job: RunJob }) {
  const failed = job.conclusion === "failure" || job.conclusion === "timed_out";
  const [open, setOpen] = useState(failed);
  const [log, setLog] = useState<string[] | null>(null); // null = not loaded, [] = loaded empty

  useEffect(() => {
    if (!open || log !== null || !job.jobId) return;
    const ctrl = new AbortController();
    fetchJobLogs(owner, repo, job.jobId, ctrl.signal).then((lines) => {
      if (!ctrl.signal.aborted) setLog(lines);
    });
    return () => ctrl.abort();
  }, [open, log, owner, repo, job.jobId]);

  return (
    <div className="overflow-hidden rounded-xl border border-line bg-surface">
      <button
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-3 border-b border-line px-5 py-3.5 text-left"
        aria-expanded={open}
      >
        <ChevronRight className={`text-muted transition-transform ${open ? "rotate-90" : ""}`} />
        <StepIcon status={job.status} conclusion={job.conclusion} />
        <span className="flex-1 text-[14px] font-medium text-cream">{job.name}</span>
        <StepStatusLabel status={job.status} conclusion={job.conclusion} />
        {job.htmlUrl && (
          <a
            href={job.htmlUrl}
            target="_blank"
            rel="noopener noreferrer"
            onClick={(e) => e.stopPropagation()}
            className="text-muted transition-colors hover:text-cream"
            aria-label={`open job ${job.name} on github`}
          >
            <ExternalLink />
          </a>
        )}
      </button>

      <ol className="divide-y divide-line">
        {job.steps.map((s, si) => (
          <li key={si} className="flex items-center gap-3 px-5 py-2.5 text-[13px]">
            <StepIcon status={s.status} conclusion={s.conclusion} />
            <span className="flex-1 text-body">{s.name}</span>
            <StepStatusLabel status={s.status} conclusion={s.conclusion} />
          </li>
        ))}
        {job.steps.length === 0 && (
          <li className="px-5 py-2.5 text-[12.5px] text-muted">no steps reported yet</li>
        )}
      </ol>

      {open && (
        <div className="border-t border-line">
          <div className="flex items-center justify-between px-5 py-2.5">
            <span className="flex items-center gap-2 text-[12px] text-muted">
              <Terminal className="text-faint" /> job log
            </span>
            <button
              onClick={() => setLog(null)}
              className="flex items-center gap-1.5 text-[12px] text-lime transition-colors hover:text-cream"
            >
              <Refresh /> refresh
            </button>
          </div>
          <div className="max-h-80 overflow-y-auto overscroll-contain bg-black/40 px-5 py-4 font-mono text-[12px] leading-6">
            <JobLogLines lines={log} />
          </div>
        </div>
      )}
    </div>
  );
}

// JobLogLines renders a job's raw log: each line leads with its emphasised date
// and time, GitHub's ##[group] markers become muted section labels, and
// ##[error]/##[warning] lines are coloured so a failure stands out.
function JobLogLines({ lines }: { lines: string[] | null }) {
  if (lines === null) return <p className="text-muted">reading the job log…</p>;
  if (lines.length === 0) return <p className="text-muted">no log output for this job.</p>;
  return (
    <>
      {lines.map((raw, i) => {
        const { ts, text } = splitLogTimestamp(raw);
        const { kind, text: clean } = classifyJobLogLine(text);
        if (kind === "group" && clean === "") return null; // drop endgroup markers
        const tone =
          kind === "error"
            ? "text-red-400"
            : kind === "warning"
              ? "text-amber-300"
              : kind === "group"
                ? "text-muted font-semibold uppercase tracking-wide text-[11px]"
                : kind === "command"
                  ? "text-blue"
                  : "text-body";
        return (
          <p key={i} className={`whitespace-pre-wrap break-words ${tone}`}>
            {ts && <span className="mr-3 select-none font-semibold text-cream">{formatLogTimestamp(ts)}</span>}
            {clean}
          </p>
        );
      })}
    </>
  );
}

// LogsViewer fetches and shows the open server's running container logs in a
// dark, scrollable terminal panel, with a refresh control and an empty state.
function LogsViewer({ serverId, ready, active }: { serverId: string; ready: boolean; active: boolean }) {
  // null = not read yet (shows "reading…"); [] = read, but no logs.
  const [lines, setLines] = useState<string[] | null>(null);
  // Tracks an in-flight refresh triggered from the button (an event handler).
  const [refreshing, setRefreshing] = useState(false);
  // The picked service chip: "" = all containers, otherwise the container name
  // (which on a compose stack is "<project>-<service>-<index>"). One container
  // per chip in the row above the log body, so the operator can zoom into the
  // app's own service instead of reading every line a compose stack prints.
  const [pickedService, setPickedService] = useState<string>("");
  const scrollRef = useRef<HTMLDivElement>(null);

  // Read once the section is open (not just the card), so collapsed logs do not
  // pull over SSH until the operator expands them. setState only happens once
  // the fetch resolves, so the effect never sets state synchronously.
  useEffect(() => {
    if (!serverId || !active || lines !== null) return;
    const ctrl = new AbortController();
    fetchServerLogs(serverId, 200, ctrl.signal).then((res) => {
      if (!ctrl.signal.aborted) setLines(res?.lines ?? []);
    });
    return () => ctrl.abort();
  }, [serverId, active, lines]);

  // Manual refresh from the button (an event handler, not an effect).
  async function refresh() {
    if (!serverId || refreshing) return;
    setRefreshing(true);
    const res = await fetchServerLogs(serverId, 200);
    setLines(res?.lines ?? []);
    setRefreshing(false);
  }

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [lines, pickedService]);

  // Containers found in the log output, in the order they first appear. The
  // chip row below lets the operator filter to one service's lines (the app's
  // own output, instead of every container in the compose stack).
  const containers: string[] = [];
  if (lines) {
    for (const line of lines) {
      if (isLogHeader(line)) {
        const name = logHeaderName(line);
        if (!containers.includes(name)) containers.push(name);
      }
    }
  }

  // Filter lines to the picked container's group: include the header itself
  // plus every line up to the next header. With no pick, show everything.
  const visibleLines: string[] = [];
  if (lines) {
    let inPicked = pickedService === "";
    for (const line of lines) {
      if (isLogHeader(line)) {
        inPicked = pickedService === "" || logHeaderName(line) === pickedService;
      }
      if (inPicked) visibleLines.push(line);
    }
  }

  return (
    <div className="mt-4 overflow-hidden rounded-xl border border-line bg-black/40">
      <div className="flex items-center justify-between border-b border-line px-5 py-3">
        <span className="flex items-center gap-2 text-[12.5px] text-muted">
          <Terminal className="text-faint" /> {pickedService === "" ? "running containers" : pickedService} · last 200 lines
        </span>
        <button
          onClick={() => void refresh()}
          disabled={refreshing || !ready}
          className="flex items-center gap-1.5 rounded-md border border-line px-2.5 py-1 text-[12px] text-lime transition-colors hover:bg-lime/10 disabled:opacity-50"
        >
          <Refresh className={refreshing ? "animate-spin" : ""} /> {refreshing ? "reading…" : "refresh"}
        </button>
      </div>
      {/* Service chips: "all" + each running container. Same segmented-control
          pattern as the home page filter chips, so an operator can switch from
          the full container stack to one service's lines and back. */}
      {containers.length > 1 && (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-line px-5 py-3">
          <span className="text-[11px] font-medium uppercase tracking-wide text-muted">log</span>
          <span className="inline-flex flex-wrap items-center gap-1 rounded-lg border border-line bg-surface p-1">
            <button
              onClick={() => setPickedService("")}
              className={`rounded-md px-3 py-1 text-[12px] font-medium transition-colors ${
                pickedService === ""
                  ? "bg-lime/15 text-lime"
                  : "text-muted hover:bg-surface-2 hover:text-cream"
              }`}
            >
              every container
            </button>
            {containers.map((name) => (
              <button
                key={name}
                onClick={() => setPickedService(name)}
                className={`rounded-md px-3 py-1 text-[12px] font-medium transition-colors ${
                  pickedService === name
                    ? "bg-lime/15 text-lime"
                    : "text-muted hover:bg-surface-2 hover:text-cream"
                }`}
              >
                {name}
              </button>
            ))}
          </span>
        </div>
      )}
      <div
        ref={scrollRef}
        className="h-72 overflow-y-auto overscroll-contain px-5 py-4 font-mono text-[12px] leading-6"
      >
        {!ready ? (
          <p className="text-muted">this server is not set up yet, so it has no container logs.</p>
        ) : lines === null ? (
          <p className="text-muted">reading container logs…</p>
        ) : lines.length === 0 ? (
          <p className="text-muted">no logs yet. once a container is running its output shows here.</p>
        ) : visibleLines.length === 0 ? (
          <p className="text-muted">no lines for {pickedService} yet.</p>
        ) : (
          visibleLines.map((line, i) => {
            if (isLogHeader(line)) {
              // When the operator has zoomed in on one service, the header
              // becomes redundant (the chip already names it), so it is
              // suppressed and only the lines below it render.
              if (pickedService !== "") return null;
              return (
                <p key={i} className="mt-3 first:mt-0 text-[11px] font-semibold uppercase tracking-wide text-lime">
                  {logHeaderName(line)}
                </p>
              );
            }
            const { ts, text } = splitLogTimestamp(line);
            return (
              <p key={i} className="whitespace-pre-wrap break-words text-body">
                {ts && (
                  <span className="mr-3 select-none font-semibold text-cream">{formatLogTimestamp(ts)}</span>
                )}
                {text}
              </p>
            );
          })
        )}
      </div>
    </div>
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

// Splits an "owner/repo" string into its parts for the run-steps fetch.
function splitRepo(full: string): [string, string] {
  const slash = full.indexOf("/");
  if (slash < 0) return ["", full];
  return [full.slice(0, slash), full.slice(slash + 1)];
}

function Summary({
  value,
  label,
  tone = "cream",
}: {
  value: string;
  label: string;
  tone?: "cream" | "blue" | "red" | "muted";
}) {
  const color =
    tone === "blue"
      ? "text-blue"
      : tone === "red"
        ? "text-red-400"
        : tone === "muted"
          ? "text-muted"
          : "text-cream";
  return (
    <span className="flex items-baseline gap-2.5">
      <span className={`text-4xl font-bold ${color}`}>{value}</span>
      <span className="text-[13px] text-muted">{label}</span>
    </span>
  );
}

function RunStrip({ runs }: { runs: DeployRun[] }) {
  return (
    <span className="hidden items-center gap-1 md:flex" title="recent deploys">
      {runs.map((r) => (
        <span
          key={r.sha}
          className={`h-5 w-1.5 rounded-sm ${runColor[r.status]} ${
            r.status === "running" ? "animate-pulse" : ""
          }`}
        />
      ))}
    </span>
  );
}

function BigMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface px-5 py-4">
      <p className="text-[11px] uppercase tracking-wide text-muted">{label}</p>
      <p className="mt-1.5 text-[18px] font-semibold text-cream">{value}</p>
    </div>
  );
}

function LatestRunPill({ status }: { status: RunStatus }) {
  const tone = status === "success" ? "blue" : status === "failed" ? "red" : "lime";
  const label = status === "success" ? "deploy ok" : status === "failed" ? "deploy failed" : "deploying";
  return <Badge tone={tone}>{label}</Badge>;
}

function DeployStatusBadge({ status }: { status?: RunStatus }) {
  if (!status) {
    return (
      <span className="flex h-11 w-11 items-center justify-center rounded-full border border-line text-muted">
        <CircleDot />
      </span>
    );
  }
  if (status === "success") {
    return (
      <span className="flex h-11 w-11 items-center justify-center rounded-full border border-blue/40 text-blue">
        <CircleCheck />
      </span>
    );
  }
  if (status === "failed") {
    return (
      <span className="flex h-11 w-11 items-center justify-center rounded-full border border-red-500/40 text-red-400">
        <CircleX />
      </span>
    );
  }
  return (
    <span className="flex h-11 w-11 items-center justify-center rounded-full border border-lime/40 text-lime">
      <CircleDot className="animate-pulse" />
    </span>
  );
}

function RunRow({ run }: { run: DeployRun }) {
  const tone = run.status === "success" ? "blue" : run.status === "failed" ? "red" : "lime";
  return (
    <li className="flex items-center gap-3 px-5 py-3 text-[13px]">
      <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${runColor[run.status]}`} />
      {run.commitUrl ? (
        <a
          href={run.commitUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="shrink-0 font-mono text-cream transition-colors hover:text-lime hover:underline"
        >
          {run.sha}
        </a>
      ) : (
        <code className="shrink-0 text-cream">{run.sha}</code>
      )}
      <span className="min-w-0 flex-1 truncate text-body">{run.message}</span>
      <Badge tone={tone}>{run.status}</Badge>
      <span className="w-16 text-right text-muted">{run.duration}</span>
      <span className="hidden w-16 text-right text-muted sm:inline">{run.when}</span>
      {run.runUrl ? (
        <a
          href={run.runUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="shrink-0 text-muted transition-colors hover:text-cream"
          aria-label={`open run ${run.sha} on github`}
        >
          <ExternalLink />
        </a>
      ) : (
        <span className="w-3.5 shrink-0" />
      )}
    </li>
  );
}

// StepIcon maps GitHub's run/step status + conclusion to a status dot/check.
function StepIcon({ status, conclusion }: { status: GithubStatus; conclusion: GithubConclusion }) {
  if (status === "completed") {
    if (conclusion === "success")
      return <CircleCheck className="shrink-0 text-blue" />;
    if (conclusion === "failure" || conclusion === "timed_out")
      return <CircleX className="shrink-0 text-red-400" />;
    // cancelled / skipped / other
    return <CircleDot className="shrink-0 text-muted" />;
  }
  if (status === "in_progress")
    return <CircleDot className="shrink-0 animate-pulse text-lime" />;
  // queued / unknown
  return <CircleDot className="shrink-0 text-faint" />;
}

// StepStatusLabel is the short, plain-English status next to a job/step.
function StepStatusLabel({ status, conclusion }: { status: GithubStatus; conclusion: GithubConclusion }) {
  let label: string;
  let cls: string;
  if (status === "completed") {
    if (conclusion === "success") {
      label = "done";
      cls = "text-blue";
    } else if (conclusion === "failure" || conclusion === "timed_out") {
      label = "failed";
      cls = "text-red-400";
    } else if (conclusion === "skipped") {
      label = "skipped";
      cls = "text-muted";
    } else if (conclusion === "cancelled") {
      label = "cancelled";
      cls = "text-muted";
    } else {
      label = "done";
      cls = "text-muted";
    }
  } else if (status === "in_progress") {
    label = "running";
    cls = "text-lime";
  } else if (status === "queued") {
    label = "queued";
    cls = "text-faint";
  } else {
    label = "waiting";
    cls = "text-faint";
  }
  return <span className={`shrink-0 text-[12px] ${cls}`}>{label}</span>;
}
