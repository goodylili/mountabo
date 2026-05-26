"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Badge } from "@/components/badge";
import { ServerAvatar } from "@/components/server-avatar";
import { RepoTreePicker } from "@/components/repo-tree-picker";
import { GithubMark, Plus, Branch as BranchIcon } from "@/components/icons";
import type { Server, Source } from "@/lib/data";
import { type DetectedPort, fetchDetectedPorts, normalizeDir } from "@/lib/ports";
import {
  type DeployConfig,
  type EnvVar,
  deploySecrets,
  generateDeployScript,
  generateWorkflow,
  mergeEnv,
  parseEnvFile,
  workflowPath,
} from "@/lib/deploy-template";

type Tab = "workflow" | "script" | "secrets";

export function ConfigureView({
  source,
  server,
  branch,
  account,
}: {
  source: Source;
  server: Server;
  branch: string;
  account: string | null;
}) {
  const [app, setApp] = useState(source.name);
  const [rootDir, setRootDir] = useState("./");
  const [deployDir, setDeployDir] = useState(`/opt/${source.name}`);
  // Ports are detected from the repo's own compose file / Dockerfile, never
  // assumed. `detected` is derived from the resolved result keyed to the current
  // inputs: while the stored result is for a stale key it reads back as null
  // ("still detecting"), so changing inputs shows the detecting state without a
  // synchronous setState in the effect body. [] means nothing to show.
  const portsKey = `${source.owner}/${source.name}@${branch}:${normalizeDir(rootDir)}`;
  const [portResult, setPortResult] = useState<{ key: string; ports: DetectedPort[] } | null>(null);
  const detected = portResult && portResult.key === portsKey ? portResult.ports : null;
  const [portValues, setPortValues] = useState<Record<string, string>>({});
  const [envVars, setEnvVars] = useState<EnvVar[]>([{ key: "", value: "" }]);
  const [tab, setTab] = useState<Tab>("workflow");
  const [showPaste, setShowPaste] = useState(false);
  const [paste, setPaste] = useState("");
  const [imported, setImported] = useState<number | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  function importEnv(text: string) {
    const parsed = parseEnvFile(text);
    setEnvVars((rows) => mergeEnv(rows, parsed));
    setImported(parsed.length);
  }

  // Detect the project's ports whenever the repo, branch, or root directory
  // changes. Seed each editable port's value from the compose default the first
  // time we see it, leaving any value the user has since typed untouched.
  useEffect(() => {
    const ctrl = new AbortController();
    fetchDetectedPorts(source.owner, source.name, branch, normalizeDir(rootDir), ctrl.signal)
      .then((ports) => {
        setPortResult({ key: portsKey, ports });
        setPortValues((prev) => {
          const next = { ...prev };
          for (const p of ports) {
            if (p.editable && !(p.envVar in next)) next[p.envVar] = p.host;
          }
          return next;
        });
      })
      .catch((e: unknown) => {
        if ((e as { name?: string })?.name !== "AbortError") setPortResult({ key: portsKey, ports: [] });
      });
    return () => ctrl.abort();
  }, [source.owner, source.name, branch, rootDir, portsKey]);

  // The host ports mountabo will set: the editable (env-var-backed) detected
  // ports, each carrying the user's value or the compose default.
  const editablePorts = useMemo(
    () =>
      (detected ?? [])
        .filter((p) => p.editable)
        .map((p) => ({ envVar: p.envVar, value: portValues[p.envVar] ?? p.host })),
    [detected, portValues],
  );

  const cfg: DeployConfig = useMemo(
    () => ({
      app: app.trim() || source.name,
      owner: source.owner,
      repo: source.name,
      branch,
      rootDir,
      deployDir,
      ports: editablePorts,
      envVars,
    }),
    [app, source.owner, source.name, branch, rootDir, deployDir, editablePorts, envVars],
  );

  const workflow = useMemo(() => generateWorkflow(cfg), [cfg]);
  const script = useMemo(() => generateDeployScript(cfg), [cfg]);
  const secrets = useMemo(() => deploySecrets(cfg), [cfg]);

  function setEnv(i: number, patch: Partial<EnvVar>) {
    setEnvVars((rows) => rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)));
  }

  return (
    <main className="mx-auto grid w-full max-w-[1400px] flex-1 grid-cols-1 gap-x-12 gap-y-10 px-4 py-8 sm:px-6 lg:grid-cols-[1fr_1fr] lg:px-8 lg:py-10">
      {/* ── left: the walkthrough form ── */}
      <div className="rise flex flex-col">
        <p className="label">step 02 / 02 · configure</p>
        <h1 className="mt-5 text-3xl font-extrabold leading-[1.1] tracking-tight text-cream sm:text-4xl sm:leading-[1.05]">
          configure your deployment.
        </h1>

        <div className="mt-5 rounded-lg border border-line bg-surface px-4 py-3 text-[13px]">
          <span className="text-muted">importing from</span>
          <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-cream">
            <GithubMark />
            {source.owner}/{source.name}
            <span className="flex items-center gap-1 text-muted">
              <BranchIcon /> {branch}
            </span>
            <span className="ml-auto flex items-center gap-2 text-muted">
              → <ServerAvatar seed={server.name} /> {server.name}
            </span>
          </div>
        </div>

        <Section label="project name">
          <input value={app} onChange={(e) => setApp(e.target.value)} className={inputCls} placeholder={source.name} />
        </Section>

        <Section label="root directory" hint="where docker-compose.yml lives">
          <RepoTreePicker
            mode="dir"
            owner={source.owner}
            repo={source.name}
            gitRef={branch}
            value={rootDir}
            onChange={setRootDir}
            placeholder="./"
          />
        </Section>

        <Section label="deploy directory" hint="on the server">
          <input value={deployDir} onChange={(e) => setDeployDir(e.target.value)} className={inputCls} placeholder={`/opt/${source.name}`} />
        </Section>

        {(detected === null || detected.length > 0) && (
          <Section label="ports" hint="detected from your project">
            {detected === null ? (
              <p className="text-[12px] text-muted">detecting ports from docker-compose.yml...</p>
            ) : (
              <div className="grid grid-cols-2 gap-3">
                {detected.map((p, i) => (
                  <label key={`${p.service}-${p.envVar}-${p.container}-${i}`} className="block">
                    <span className="mb-1 block text-[11px] text-muted">
                      {p.editable ? p.envVar : p.service || `port ${p.container}`}
                    </span>
                    {p.editable ? (
                      <input
                        value={portValues[p.envVar] ?? p.host}
                        onChange={(e) => setPortValues((v) => ({ ...v, [p.envVar]: e.target.value }))}
                        inputMode="numeric"
                        className={inputCls}
                      />
                    ) : (
                      <div className={`${inputCls} flex items-center justify-between`} title="fixed in your compose file">
                        <span>{p.host || "auto"}</span>
                        <span className="text-[11px] text-faint">fixed</span>
                      </div>
                    )}
                  </label>
                ))}
              </div>
            )}
          </Section>
        )}

        <Section label="environment variables" hint="become encrypted repo secrets">
          {/* Import a whole .env at once: paste it or pick a file. Parsing is
              local; values never leave the browser here. */}
          <div className="mb-3 flex items-center gap-2">
            <button
              onClick={() => fileRef.current?.click()}
              className="rounded-md border border-line px-2.5 py-1.5 text-[12px] text-muted transition-colors hover:border-line-strong hover:text-cream"
            >
              import .env file
            </button>
            <button
              onClick={() => setShowPaste((v) => !v)}
              className="rounded-md border border-line px-2.5 py-1.5 text-[12px] text-muted transition-colors hover:border-line-strong hover:text-cream"
            >
              paste .env
            </button>
            {imported !== null && (
              <span className="text-[12px] text-lime">imported {imported}</span>
            )}
            <input
              ref={fileRef}
              type="file"
              accept=".env,.txt,text/plain"
              className="hidden"
              onChange={async (e) => {
                const f = e.target.files?.[0];
                if (f) importEnv(await f.text());
                e.target.value = ""; // allow re-importing the same file
              }}
            />
          </div>

          {showPaste && (
            <div className="mb-3">
              <textarea
                value={paste}
                onChange={(e) => setPaste(e.target.value)}
                rows={5}
                spellCheck={false}
                placeholder={"DATABASE_URL=postgres://...\nAPI_KEY=sk-..."}
                className={`${inputCls} resize-y font-mono`}
              />
              <div className="mt-2 flex items-center gap-2">
                <button
                  onClick={() => {
                    importEnv(paste);
                    setPaste("");
                    setShowPaste(false);
                  }}
                  disabled={!paste.trim()}
                  className="rounded-md border border-lime/40 px-3 py-1.5 text-[12px] text-lime transition-colors hover:bg-lime/10 disabled:opacity-40"
                >
                  parse &amp; add
                </button>
                <span className="text-[11px] text-muted">KEY=value per line · # comments and quotes handled</span>
              </div>
            </div>
          )}

          <div className="space-y-2">
            {envVars.map((row, i) => (
              <div key={i} className="flex items-center gap-2">
                <input
                  value={row.key}
                  onChange={(e) => setEnv(i, { key: e.target.value })}
                  placeholder="KEY"
                  className={`${inputCls} min-w-0 flex-1 font-mono`}
                />
                <input
                  value={row.value}
                  onChange={(e) => setEnv(i, { value: e.target.value })}
                  placeholder="value"
                  type="password"
                  className={`${inputCls} min-w-0 flex-1`}
                />
                <button
                  onClick={() => setEnvVars((rows) => (rows.length > 1 ? rows.filter((_, idx) => idx !== i) : rows))}
                  className="shrink-0 rounded-md border border-line px-2.5 py-2 text-[12px] text-muted transition-colors hover:border-red-400/50 hover:text-red-300"
                  aria-label="remove"
                >
                  ×
                </button>
              </div>
            ))}
            <button
              onClick={() => setEnvVars((rows) => [...rows, { key: "", value: "" }])}
              className="flex items-center gap-1.5 text-[12px] text-lime transition-colors hover:text-cream"
            >
              <Plus /> add variable
            </button>
          </div>
        </Section>

        {/* CTA: this iteration generates the preview; GitHub writes land next. */}
        <div className="mt-8">
          <button
            disabled
            className="flex w-full items-center justify-center gap-2 rounded-xl border border-line bg-surface-2 px-6 py-4 font-bold text-muted"
            title="GitHub writes (deploy key, secrets, workflow) are the next step"
          >
            deploy (writes land next)
          </button>
          <p className="mt-2 text-center text-[11px] text-muted">
            preview is live on the right. the next pass adds the deploy key, sets {secrets.length} secrets, and
            commits these files.
          </p>
        </div>
      </div>

      {/* ── right: live preview ── */}
      <div className="rise flex flex-col" style={{ animationDelay: "80ms" }}>
        <div className="flex items-center gap-1 text-[12px]">
          <PreviewTab active={tab === "workflow"} onClick={() => setTab("workflow")}>
            {workflowPath(cfg).split("/").pop()}
          </PreviewTab>
          <PreviewTab active={tab === "script"} onClick={() => setTab("script")}>
            deploy.sh
          </PreviewTab>
          <PreviewTab active={tab === "secrets"} onClick={() => setTab("secrets")}>
            secrets ({secrets.length})
          </PreviewTab>
        </div>

        <div className="mt-3 flex-1">
          {tab === "workflow" && <Code path={workflowPath(cfg)} body={workflow} />}
          {tab === "script" && <Code path="deploy.sh" body={script} />}
          {tab === "secrets" && (
            <div className="overflow-hidden rounded-xl border border-line bg-surface">
              <div className="border-b border-line px-5 py-3 text-[12px] text-muted">
                set on {source.owner}/{source.name} · actions secrets
              </div>
              <ul>
                {secrets.map((s) => (
                  <li key={s.name} className="flex items-center justify-between border-b border-line px-5 py-3 text-[13px]">
                    <code className="text-cream">{s.name}</code>
                    <Badge tone={s.managed === "mountabo" ? "blue" : "lime"}>
                      {s.managed === "mountabo" ? "auto" : "your value"}
                    </Badge>
                  </li>
                ))}
              </ul>
              <p className="px-5 py-3 text-[12px] leading-5 text-muted">
                <code className="text-cream">SERVER_*</code> are filled from the selected server + its mountabo key.
                your env vars are encrypted (sealed box) before upload; values never leave your machine in the clear.
              </p>
            </div>
          )}
        </div>
        {account && <p className="mt-2 text-[11px] text-faint">connected as {account}</p>}
      </div>
    </main>
  );
}

const inputCls =
  "w-full rounded-lg border border-line bg-surface-2 px-3 py-2.5 text-[13px] text-cream placeholder:text-muted focus:border-line-strong focus:outline-none";

function Section({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="mt-6">
      <div className="mb-2 flex items-center justify-between">
        <span className="label">{label}</span>
        {hint && <span className="text-[11px] text-muted">{hint}</span>}
      </div>
      {children}
    </div>
  );
}

function PreviewTab({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      className={`rounded-md px-3 py-1.5 font-mono transition-colors ${
        active ? "bg-surface-2 text-cream" : "text-muted hover:text-cream"
      }`}
    >
      {children}
    </button>
  );
}

function Code({ path, body }: { path: string; body: string }) {
  return (
    <div className="overflow-hidden rounded-xl border border-line bg-surface">
      <div className="flex items-center justify-between border-b border-line px-4 py-2.5 text-[12px]">
        <code className="text-muted">{path}</code>
        <Badge tone="lime">will write</Badge>
      </div>
      <pre className="max-h-[68vh] overflow-auto px-4 py-4 text-[12px] leading-[1.7] text-body">
        <code>{body}</code>
      </pre>
    </div>
  );
}
