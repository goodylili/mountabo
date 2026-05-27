"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Badge } from "@/components/badge";
import { ServerAvatar } from "@/components/server-avatar";
import { RepoTreePicker } from "@/components/repo-tree-picker";
import { StreamLog } from "@/components/stream-log";
import { GithubMark, Plus, Branch as BranchIcon } from "@/components/icons";
import type { Server, Source } from "@/lib/data";
import { type DetectedPort, fetchDetectedPorts, normalizeDir } from "@/lib/ports";
import { fetchListeningPorts } from "@/lib/server-ports";
import { type DeployPreview, type PreviewPort, type PreviewRequest, fetchPreview } from "@/lib/deploy-preview";
import { addEnvKeys, type EnvVar, mergeEnv, parseEnvFile } from "@/lib/deploy-template";
import { fetchEnvExampleKeys } from "@/lib/env-example";

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
  const [portResult, setPortResult] = useState<{ key: string; strategy: string; ports: DetectedPort[] } | null>(null);
  const detected = portResult && portResult.key === portsKey ? portResult.ports : null;
  // Deploy strategy detected from the repo: "compose" (has a Compose file) or
  // "docker" (only a Dockerfile). In docker mode each EXPOSE'd port gets an
  // editable host port; in compose mode only env-var-backed ports are editable.
  const strategy = portResult && portResult.key === portsKey ? portResult.strategy : "";
  const isDocker = strategy === "docker";
  const [portValues, setPortValues] = useState<Record<string, string>>({});
  const [envVars, setEnvVars] = useState<EnvVar[]>([{ key: "", value: "" }]);
  const [tab, setTab] = useState<Tab>("workflow");
  const [showPaste, setShowPaste] = useState(false);
  const [paste, setPaste] = useState("");
  const [imported, setImported] = useState<number | null>(null);
  // Env var values render masked by default (they are secrets); this toggle
  // reveals them so the operator can check what they pasted before deploying.
  const [showValues, setShowValues] = useState(false);
  // Variable names found in the repo's .env.example, used to pre-fill the rows.
  const [exampleKeys, setExampleKeys] = useState<string[]>([]);
  const fileRef = useRef<HTMLInputElement>(null);

  // Ports already listening on the selected server. A host port in this set is
  // flagged as a collision in the form (but never changed, the operator decides).
  const [busyPorts, setBusyPorts] = useState<{ id: string; ports: Set<number> } | null>(null);
  const busy = busyPorts && busyPorts.id === server.id ? busyPorts.ports : null;

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
      .then((res) => {
        setPortResult({ key: portsKey, strategy: res.strategy, ports: res.ports });
        setPortValues((prev) => {
          const next = { ...prev };
          for (const p of res.ports) {
            const key = p.envVar || p.container; // env var (compose) or container port (docker)
            if (key && !(key in next)) next[key] = p.host || p.container;
          }
          return next;
        });
      })
      .catch((e: unknown) => {
        if ((e as { name?: string })?.name !== "AbortError") setPortResult({ key: portsKey, strategy: "", ports: [] });
      });
    return () => ctrl.abort();
  }, [source.owner, source.name, branch, rootDir, portsKey]);

  // Discover the repo's .env.example variable names whenever the repo, branch,
  // or root directory changes. When the editor is still pristine (its lone empty
  // row), auto-fill the discovered keys so the operator just fills the values,
  // the way Vercel pre-populates from a project's example. We only auto-fill
  // when pristine so switching branch never clobbers typed or imported vars; the
  // "use repo .env.example" button below re-adds any missing keys on demand.
  useEffect(() => {
    const ctrl = new AbortController();
    fetchEnvExampleKeys(source.owner, source.name, branch, normalizeDir(rootDir), ctrl.signal)
      .then((keys) => {
        setExampleKeys(keys);
        if (keys.length === 0) return;
        setEnvVars((rows) => {
          const pristine = rows.length === 1 && !rows[0].key.trim() && !rows[0].value.trim();
          return pristine ? addEnvKeys(rows, keys) : rows;
        });
      })
      .catch(() => {}); // includes AbortError when the repo/branch changes
    return () => ctrl.abort();
  }, [source.owner, source.name, branch, rootDir]);

  // Load the ports already in use on the selected server so the form can flag a
  // host port that would collide. Keyed to the server id so a stale set reads
  // back as null until the new server's ports load.
  useEffect(() => {
    const ctrl = new AbortController();
    fetchListeningPorts(server.id, ctrl.signal)
      .then((ports) => setBusyPorts({ id: server.id, ports: new Set(ports) }))
      .catch(() => setBusyPorts({ id: server.id, ports: new Set() }));
    return () => ctrl.abort();
  }, [server.id]);

  // The ports mountabo will set, in the shape the backend expects. In docker
  // mode every EXPOSE'd port gets an editable host port (default = container
  // port); in compose mode only env-var-backed ports are mountabo's to set.
  const configuredPorts: PreviewPort[] = useMemo(() => {
    const ports = detected ?? [];
    if (isDocker) {
      return ports.map((p) => ({
        envVar: "",
        value: portValues[p.container] ?? p.host ?? p.container,
        container: p.container,
      }));
    }
    return ports
      .filter((p) => p.editable)
      .map((p) => ({ envVar: p.envVar, value: portValues[p.envVar] ?? p.host, container: p.container }));
  }, [detected, isDocker, portValues]);

  // The preview (workflow + deploy.sh + secrets) is generated by the backend,
  // the single source of truth, so it always matches what a deploy commits. The
  // request is debounced as the operator edits.
  const [preview, setPreview] = useState<DeployPreview | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  // Snapshot of the config sent when the operator clicks deploy; while set, the
  // live deploy stream is shown. Stable so the stream runs exactly once.
  const [deployBody, setDeployBody] = useState<PreviewRequest | null>(null);
  const [deployed, setDeployed] = useState(false);

  // Any non-empty env var key that isn't a valid name blocks deploy.
  const envInvalid = envVars.some((v) => {
    const k = v.key.trim();
    return k !== "" && !envNamePattern.test(k);
  });

  const previewReq: PreviewRequest = useMemo(
    () => ({
      app: app.trim() || source.name,
      owner: source.owner,
      repo: source.name,
      branch,
      strategy: strategy || "compose",
      rootDir,
      deployDir,
      ports: configuredPorts,
      // Only valid, named vars go to the backend; invalid keys are flagged
      // inline and block deploy below.
      envVars: envVars.filter((v) => {
        const k = v.key.trim();
        return k !== "" && envNamePattern.test(k);
      }),
    }),
    [app, source.owner, source.name, branch, strategy, rootDir, deployDir, configuredPorts, envVars],
  );

  useEffect(() => {
    const ctrl = new AbortController();
    const timer = setTimeout(() => {
      fetchPreview(previewReq, ctrl.signal)
        .then((res) => {
          if ("error" in res) {
            setPreviewError(res.error);
          } else {
            setPreview(res);
            setPreviewError(null);
          }
        })
        .catch((e: unknown) => {
          if ((e as { name?: string })?.name !== "AbortError") setPreviewError("preview failed");
        });
    }, 300);
    return () => {
      clearTimeout(timer);
      ctrl.abort();
    };
  }, [previewReq]);

  function setEnv(i: number, patch: Partial<EnvVar>) {
    setEnvVars((rows) => rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)));
  }

  return (
    <main className="mx-auto grid w-full max-w-[1100px] flex-1 grid-cols-1 gap-x-12 gap-y-10 px-4 py-8 sm:px-6 lg:grid-cols-[1fr_1fr] lg:px-8 lg:py-10">
      {/* ── left: the walkthrough form ── */}
      <div className="rise flex flex-col">
        <p className="label">step 02 / 02 · configure</p>
        <h1 className="mt-5 text-4xl font-extrabold leading-[1.05] tracking-tight text-cream sm:text-5xl sm:leading-[1.02]">
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
              <p className="text-[12px] text-muted">detecting ports from your project...</p>
            ) : (
              <div className="grid grid-cols-2 gap-3">
                {detected.map((p, i) => {
                  // Value key: env var (compose) or container port (docker).
                  const key = p.envVar || p.container;
                  const editable = isDocker || p.editable; // docker: every EXPOSE'd port is editable
                  const value = portValues[key] ?? p.host ?? p.container;
                  const occupied = editable && !!busy && value.trim() !== "" && busy.has(Number(value));
                  const label = isDocker
                    ? `host port for :${p.container}`
                    : p.editable
                      ? p.envVar
                      : p.service || `port ${p.container}`;
                  return (
                    <label key={`${p.service}-${p.envVar}-${p.container}-${i}`} className="block">
                      <span className="mb-1 flex items-center gap-1.5 text-[11px] text-muted">
                        {label}
                        {occupied && (
                          <span className="text-red-300" title={`${value} is already in use on ${server.name}`}>
                            {"\u{1F6AB}"} in use on the server
                          </span>
                        )}
                      </span>
                      {editable ? (
                        <input
                          value={value}
                          onChange={(e) => setPortValues((v) => ({ ...v, [key]: e.target.value }))}
                          inputMode="numeric"
                          aria-invalid={occupied}
                          className={`${inputCls} ${occupied ? "border-red-400/60" : ""}`}
                        />
                      ) : (
                        <div className={`${inputCls} flex items-center justify-between`} title="fixed in your compose file">
                          <span>{p.host || "auto"}</span>
                          <span className="text-[11px] text-faint">fixed</span>
                        </div>
                      )}
                    </label>
                  );
                })}
              </div>
            )}
          </Section>
        )}

        <Section label="environment variables" hint="become encrypted repo secrets">
          <p className="mb-3 text-[12px] leading-5 text-muted">
            your app&apos;s own variables only. mountabo sets the server connection (host, ssh key, deploy
            directory) for you, so you never add server details like an ip or password here.
          </p>
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
            <button
              onClick={() => setShowValues((v) => !v)}
              className="rounded-md border border-line px-2.5 py-1.5 text-[12px] text-muted transition-colors hover:border-line-strong hover:text-cream"
            >
              {showValues ? "hide values" : "show values"}
            </button>
            {exampleKeys.length > 0 && (
              <button
                onClick={() => setEnvVars((rows) => addEnvKeys(rows, exampleKeys))}
                className="rounded-md border border-lime/40 px-2.5 py-1.5 text-[12px] text-lime transition-colors hover:bg-lime/10"
              >
                use repo .env.example ({exampleKeys.length})
              </button>
            )}
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
            {envVars.map((row, i) => {
              const key = row.key.trim();
              const badKey = key !== "" && !envNamePattern.test(key);
              return (
                <div key={i} className="flex items-start gap-2">
                  <div className="min-w-0 flex-1">
                    <input
                      value={row.key}
                      onChange={(e) => setEnv(i, { key: e.target.value })}
                      placeholder="KEY"
                      aria-invalid={badKey}
                      className={`${inputCls} w-full font-mono ${badKey ? "border-red-400/60" : ""}`}
                    />
                    {badKey && (
                      <span className="mt-1 block text-[11px] text-red-300">
                        not a valid name. use letters, digits and underscores, and don&apos;t start with a digit, for
                        example DATABASE_URL or API_KEY.
                      </span>
                    )}
                  </div>
                  <input
                    value={row.value}
                    onChange={(e) => setEnv(i, { value: e.target.value })}
                    placeholder="value"
                    type={showValues ? "text" : "password"}
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
              );
            })}
            <button
              onClick={() => setEnvVars((rows) => [...rows, { key: "", value: "" }])}
              className="flex items-center gap-1.5 text-[12px] text-lime transition-colors hover:text-cream"
            >
              <Plus /> add variable
            </button>
          </div>
        </Section>

        {/* CTA: commit the workflow + deploy.sh and set the secrets, streamed live. */}
        <div className="mt-8">
          <button
            disabled={!preview || envInvalid}
            onClick={() => setDeployBody(previewReq)}
            className="flex w-full items-center justify-center gap-2 rounded-xl border border-lime/40 bg-lime/10 px-6 py-4 font-bold text-lime transition-colors hover:bg-lime/20 disabled:cursor-not-allowed disabled:border-line disabled:bg-surface-2 disabled:text-muted"
            title={
              envInvalid
                ? "fix the invalid environment variable name first"
                : preview
                  ? "commit the workflow + deploy.sh and set the Actions secrets"
                  : "waiting for a valid configuration"
            }
          >
            {deployed ? "re-deploy configuration" : "deploy"}
          </button>
          <p className="mt-2 text-center text-[11px] text-muted">
            commits these files and sets {preview?.secrets.length ?? 0} secrets on {source.owner}/{source.name}. push to{" "}
            {branch} afterwards to ship.
          </p>
          {previewError && <p className="mt-2 text-center text-[11px] text-red-300">{previewError}</p>}
        </div>
      </div>

      {deployBody && (
        <StreamLog
          title={`deploying ${app.trim() || source.name}`}
          subtitle={`${source.owner}/${source.name} → ${server.name}`}
          url={`/api/servers/${server.id}/deploy`}
          body={deployBody}
          onClose={() => setDeployBody(null)}
          onDone={(ok) => {
            if (ok) setDeployed(true);
          }}
        />
      )}

      {/* ── right: live preview ── */}
      <div className="rise flex flex-col" style={{ animationDelay: "80ms" }}>
        <div className="flex items-center gap-1 text-[12px]">
          <PreviewTab active={tab === "workflow"} onClick={() => setTab("workflow")}>
            {preview ? preview.workflowPath.split("/").pop() : "workflow"}
          </PreviewTab>
          <PreviewTab active={tab === "script"} onClick={() => setTab("script")}>
            deploy.sh
          </PreviewTab>
          <PreviewTab active={tab === "secrets"} onClick={() => setTab("secrets")}>
            secrets ({preview?.secrets.length ?? 0})
          </PreviewTab>
        </div>

        <div className="mt-3 flex-1">
          {!preview ? (
            <div className="rounded-xl border border-line bg-surface px-4 py-4 text-[12px] text-muted">
              {previewError ?? "generating preview..."}
            </div>
          ) : (
            <>
              {tab === "workflow" && <Code path={preview.workflowPath} body={preview.workflow} />}
              {tab === "script" && <Code path="deploy.sh" body={preview.deployScript} />}
              {tab === "secrets" && (
                <div className="overflow-hidden rounded-xl border border-line bg-surface">
                  <div className="border-b border-line px-5 py-3 text-[12px] text-muted">
                    set on {source.owner}/{source.name} · actions secrets
                  </div>
                  <ul>
                    {preview.secrets.map((s) => (
                      <li key={s.name} className="flex items-center justify-between border-b border-line px-5 py-3 text-[13px]">
                        <code className="text-cream">{s.name}</code>
                        <Badge tone={s.managed ? "blue" : "lime"}>{s.managed ? "auto" : "your value"}</Badge>
                      </li>
                    ))}
                  </ul>
                  <p className="px-5 py-3 text-[12px] leading-5 text-muted">
                    <code className="text-cream">SERVER_*</code> are filled from the selected server + its mountabo key.
                    your env vars are encrypted (sealed box) before upload; values never leave your machine in the clear.
                  </p>
                </div>
              )}
            </>
          )}
        </div>
        {account && <p className="mt-2 text-[11px] text-faint">connected as {account}</p>}
      </div>
    </main>
  );
}

const inputCls =
  "w-full rounded-lg border border-line bg-surface-2 px-3 py-2.5 text-[13px] text-cream placeholder:text-muted focus:border-line-strong focus:outline-none";

// A valid GitHub Actions secret / env var name: letters, digits, underscores,
// not starting with a digit. Matches the backend's rule.
const envNamePattern = /^[A-Za-z_][A-Za-z0-9_]*$/;

function Section({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="mt-8">
      <div className="mb-2.5 flex items-center justify-between">
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
