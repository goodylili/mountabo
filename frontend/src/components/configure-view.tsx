"use client";

import { useEffect, useMemo, useState } from "react";
import { Badge } from "@/components/badge";
import { ServerAvatar } from "@/components/server-avatar";
import { StatusBar } from "@/components/status-bar";
import {
  ArrowRight,
  Check,
  ChevronDown,
  ChevronRight,
  GithubMark,
  Plus,
  Shield,
} from "@/components/icons";
import type { Server, Source } from "@/lib/data";
import type { DeployKeyInfo, SecretRow } from "@/lib/preview";

type Tab = "workflow" | "deploy-key" | "secrets";

function buildDefaults(source: Source) {
  switch (source.language) {
    case "go":
      return { build: `go build -o bin/${source.name} ./...`, start: `./bin/${source.name} serve` };
    case "monorepo":
      return { build: "turbo run build", start: "turbo run start" };
    case "next.js":
      return { build: "npm ci && npm run build", start: "npm run start" };
    default:
      return { build: "npm ci && npm run build", start: "npm run start" };
  }
}

export function ConfigureView({
  source,
  server,
  branch,
  account,
  yaml,
  secrets,
  deployKey,
}: {
  source: Source;
  server: Server;
  branch: string;
  account: string | null;
  yaml: string;
  secrets: SecretRow[];
  deployKey: DeployKeyInfo;
}) {
  const defaults = useMemo(() => buildDefaults(source), [source]);
  const [root, setRoot] = useState("");
  const [build, setBuild] = useState(defaults.build);
  const [start, setStart] = useState(defaults.start);
  const [tab, setTab] = useState<Tab>("secrets");
  const [reqOpen, setReqOpen] = useState(false);
  const [envOpen, setEnvOpen] = useState(false);
  const [written, setWritten] = useState(false);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = document.activeElement;
      const typing = el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement;
      if (e.key === "Enter" && !typing && !written) {
        e.preventDefault();
        setWritten(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [written]);

  const full = `${source.owner}/${source.name}`;

  return (
    <>
      <main className="mx-auto grid w-full max-w-[1400px] flex-1 grid-cols-1 gap-x-16 px-8 py-12 lg:grid-cols-[1.55fr_1fr]">
        {/* ── left: the form ── */}
        <div className="rise flex flex-col">
          <p className="label">
            step 02 / 02 · configure
          </p>
          <h1 className="mt-6 text-5xl font-extrabold leading-[1.02] tracking-tight text-cream">
            one workflow file.
            <br />
            then <span className="italic text-lime">push.</span>
          </h1>
          <p className="mt-5 max-w-md text-[14px] leading-7 text-body">
            mountabo writes <Underline>.github/workflows/mountabo-deploy.yml</Underline> to your
            repo, adds a deploy key, and stores three secrets. from there, every push to{" "}
            <Underline>{branch}</Underline> deploys.
          </p>

          <Section label="source" tag="github · read + write" tagActive>
            <Card>
              <GithubMark className="text-cream" />
              <div className="flex-1">
                <span className="text-[15px] font-medium text-cream">{full}</span>
                <span className="mt-1 block text-[12px] text-muted">
                  {branch} · {source.loc ?? "-"} · {source.language} · {source.updated}
                </span>
              </div>
              <ChevronDown className="text-muted" />
            </Card>
          </Section>

          <Section label="destination" tag="your server · ssh">
            <Card>
              <ServerAvatar seed={server.name} />
              <div className="flex-1">
                <span className="flex items-center gap-2.5 text-[15px] font-medium text-cream">
                  {server.name}
                  <Badge tone={server.status === "healthy" ? "blue" : "gray"}>{server.status}</Badge>
                </span>
                <span className="mt-1 block text-[12px] text-muted">
                  {server.provider} {server.plan} · {server.ip} · {server.region}
                </span>
              </div>
              <ChevronDown className="text-muted" />
            </Card>
          </Section>

          <Section label="build" tag={`auto-detected · ${source.language}`}>
            <div className="space-y-4">
              <Field label="root" hint="working directory">
                <input
                  value={root}
                  onChange={(e) => setRoot(e.target.value)}
                  placeholder="./ leave blank for root"
                  className="flex-1 bg-transparent text-[13px] text-cream placeholder:text-muted focus:outline-none"
                />
                <span className="text-[12px] text-lime">edit</span>
              </Field>
              <div className="grid grid-cols-2 gap-4">
                <Field label="build cmd">
                  <span className="text-muted">$</span>
                  <input
                    value={build}
                    onChange={(e) => setBuild(e.target.value)}
                    className="w-full bg-transparent text-[13px] text-cream focus:outline-none"
                  />
                </Field>
                <Field label="start cmd">
                  <span className="text-muted">$</span>
                  <input
                    value={start}
                    onChange={(e) => setStart(e.target.value)}
                    className="w-full bg-transparent text-[13px] text-cream focus:outline-none"
                  />
                </Field>
              </div>
            </div>
          </Section>

          <Section label="secrets" tag="stored in repo actions">
            <div className="space-y-3">
              <Expandable
                open={reqOpen}
                onToggle={() => setReqOpen((v) => !v)}
                title="required (auto-generated)"
                trailing={<Counter>{secrets.length}</Counter>}
              >
                <ul className="space-y-2 pt-1">
                  {secrets.map((s) => (
                    <li key={s.name} className="flex items-center justify-between text-[12.5px]">
                      <code className="text-cream">{s.name}</code>
                      <span className="text-muted">{s.value}</span>
                    </li>
                  ))}
                </ul>
              </Expandable>

              <Expandable
                open={envOpen}
                onToggle={() => setEnvOpen((v) => !v)}
                title="your env vars"
                trailing={
                  <span className="flex items-center gap-1 rounded-md border border-line px-2 py-0.5 text-[11px] text-muted">
                    <Plus /> add
                  </span>
                }
              >
                <p className="pt-1 text-[12.5px] text-muted">
                  no custom env vars. add <code className="text-cream">KEY=value</code> pairs to
                  inject them into the deploy.
                </p>
              </Expandable>
            </div>
          </Section>

          {/* CTA */}
          <div className="mt-10">
            <p className="label">→ mountabo will</p>
            <button
              onClick={() => setWritten(true)}
              className={`mt-3 flex w-full items-center justify-between rounded-xl px-6 py-4 font-bold transition-transform hover:-translate-y-0.5 ${
                written ? "bg-blue text-black" : "cta-glow bg-lime-fill text-black"
              }`}
            >
              <span className="flex items-center gap-3">
                <Check />
                {written ? "workflow written & connected" : "write workflow & connect"}
              </span>
              <kbd className="rounded-md bg-black/15 px-2 py-1 text-[12px] font-medium">↵ enter</kbd>
            </button>
            <div className="mt-3 flex items-center justify-between">
              <p className="flex items-center gap-2 text-[12px] text-muted">
                <Shield className="text-faint" />
                credentials never leave your machine
              </p>
              <button className="text-[12px] text-lime transition-colors hover:text-cream">
                ↳ dry run
              </button>
            </div>
          </div>
        </div>

        {/* ── right: the preview ── */}
        <div className="rise flex min-w-0 flex-col" style={{ animationDelay: "100ms" }}>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1 text-[12px]">
              <Tab2 active={tab === "workflow"} onClick={() => setTab("workflow")} dot>
                workflow
              </Tab2>
              <Tab2 active={tab === "deploy-key"} onClick={() => setTab("deploy-key")}>
                deploy key
              </Tab2>
              <Tab2 active={tab === "secrets"} onClick={() => setTab("secrets")}>
                secrets
              </Tab2>
            </div>
            <Badge tone="lime-solid" dot>
              {written ? "preview · written" : "preview · unwritten"}
            </Badge>
          </div>

          {/* server summary */}
          <div className="mt-5 rounded-xl border border-line bg-surface p-5">
            <div className="flex items-center gap-3">
              <ServerAvatar seed={server.name} size="lg" />
              <div className="flex-1">
                <span className="text-lg font-bold text-cream">{server.name}</span>
                <span className="mt-0.5 block text-[12px] text-muted">
                  {server.provider} {server.plan} · {server.specs.cpu} · {server.specs.ram} ·{" "}
                  {server.region}
                </span>
              </div>
              <Badge tone={server.status === "healthy" ? "blue" : "gray"} dot>
                {server.status} · {server.uptimeLabel.replace("up ", "")}
              </Badge>
            </div>
            <div className="mt-4 flex flex-wrap items-baseline gap-x-6 gap-y-2 border-t border-line pt-4 text-[13px]">
              <Meta label="ip" value={server.ip} />
              <Meta label="ssh" value={`port ${server.specs.sshPort}`} />
              <Meta label="os" value={server.specs.os} />
              <Meta label="ping" value={server.specs.ping} />
            </div>
          </div>

          {/* tab body */}
          <div className="mt-4 flex-1">
            {tab === "workflow" && <WorkflowTab yaml={yaml} />}
            {tab === "deploy-key" && <DeployKeyTab full={full} deployKey={deployKey} />}
            {tab === "secrets" && <SecretsTab secrets={secrets} written={written} account={account} full={full} />}
          </div>
        </div>
      </main>

      <StatusBar
        pill={written ? "written" : "ready to write"}
        left={[
          { label: "localhost:7777", tone: "blue" },
          { label: full },
          { label: `→ ${server.name}`, tone: "lime" },
          { label: "step 2/2" },
        ]}
        right={[
          { label: "v0.4.2" },
          { label: "✓ keychain unlocked", tone: "blue" },
          { label: "UTF-8 · LF" },
        ]}
      />
    </>
  );
}

/* ── small building blocks ── */

function Underline({ children }: { children: React.ReactNode }) {
  return <span className="text-cream underline decoration-line-strong underline-offset-4">{children}</span>;
}

function Section({
  label,
  tag,
  tagActive,
  children,
}: {
  label: string;
  tag: string;
  tagActive?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div className="mt-8">
      <div className="mb-3 flex items-center justify-between">
        <span className="flex items-center gap-2 label">
          <ChevronRight className="text-faint" /> {label}
        </span>
        <span
          className={`text-[11px] ${
            tagActive ? "rounded bg-lime/10 px-2 py-0.5 text-lime" : "text-muted"
          }`}
        >
          {tag}
        </span>
      </div>
      {children}
    </div>
  );
}

function Card({ children }: { children: React.ReactNode }) {
  return (
    <button className="flex w-full items-center gap-3 rounded-lg border border-line bg-surface px-4 py-3.5 text-left transition-colors hover:border-line-strong">
      {children}
    </button>
  );
}

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <span className="label">{label}</span>
        {hint && <span className="text-[11px] text-muted">{hint}</span>}
      </div>
      <div className="flex items-center gap-2 rounded-lg border border-line bg-surface-2 px-3.5 py-2.5">
        {children}
      </div>
    </div>
  );
}

function Expandable({
  open,
  onToggle,
  title,
  trailing,
  children,
}: {
  open: boolean;
  onToggle: () => void;
  title: string;
  trailing: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-lg border border-line bg-surface">
      <button onClick={onToggle} className="flex w-full items-center justify-between px-4 py-3 text-left">
        <span className="flex items-center gap-2 text-[13px] text-cream">
          <ChevronRight className={`text-muted transition-transform ${open ? "rotate-90" : ""}`} />
          {title}
        </span>
        {trailing}
      </button>
      {open && <div className="border-t border-line px-4 py-3">{children}</div>}
    </div>
  );
}

function Counter({ children }: { children: React.ReactNode }) {
  return (
    <span className="rounded border border-line px-1.5 py-0.5 text-[11px] text-muted">{children}</span>
  );
}

function Tab2({
  active,
  onClick,
  dot,
  children,
}: {
  active: boolean;
  onClick: () => void;
  dot?: boolean;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 transition-colors ${
        active ? "bg-surface-2 text-cream" : "text-muted hover:text-cream"
      }`}
    >
      {dot && <span className={`h-1.5 w-1.5 rounded-full ${active ? "bg-lime" : "bg-faint"}`} />}
      {children}
    </button>
  );
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <span className="flex items-baseline gap-2">
      <span className="label">{label}</span>
      <span className="text-cream">{value}</span>
    </span>
  );
}

function WorkflowTab({ yaml }: { yaml: string }) {
  return (
    <div className="overflow-hidden rounded-xl border border-line bg-surface">
      <div className="flex items-center justify-between border-b border-line px-4 py-2.5 text-[12px]">
        <code className="text-muted">.github/workflows/mountabo-deploy.yml</code>
        <Badge tone="lime">will write</Badge>
      </div>
      <pre className="overflow-x-auto px-4 py-4 text-[12px] leading-[1.7] text-body">
        <code>{yaml}</code>
      </pre>
    </div>
  );
}

function DeployKeyTab({ full, deployKey }: { full: string; deployKey: DeployKeyInfo }) {
  return (
    <div className="space-y-3">
      <div className="overflow-hidden rounded-xl border border-line bg-surface">
        <div className="flex items-center justify-between border-b border-line px-4 py-2.5 text-[12px]">
          <code className="text-muted">public key · {deployKey.algo}</code>
          <Badge tone="lime">will add</Badge>
        </div>
        <pre className="overflow-x-auto px-4 py-4 text-[12px] leading-[1.7] text-body">
          <code>{deployKey.publicKey}</code>
        </pre>
      </div>
      <div className="rounded-xl border border-line bg-surface px-4 py-3 text-[12.5px]">
        <div className="flex items-center justify-between gap-3">
          <span className="text-muted">fingerprint</span>
          <code className="min-w-0 break-all text-right text-cream">{deployKey.fingerprint}</code>
        </div>
        <p className="mt-3 leading-6 text-muted">
          added to <span className="text-cream">{full}</span> as a{" "}
          <span className="text-cream">read-only</span> deploy key. the private half is stored as
          the <code className="text-cream">SSH_PRIVATE_KEY</code> action secret: never written to
          disk in plaintext.
        </p>
      </div>
    </div>
  );
}

function SecretsTab({
  secrets,
  written,
  account,
  full,
}: {
  secrets: SecretRow[];
  written: boolean;
  account: string | null;
  full: string;
}) {
  return (
    <div className="overflow-hidden rounded-xl border border-line bg-surface">
      <div className="grid grid-cols-[1.4fr_1.4fr_auto] items-center gap-4 border-b border-line px-5 py-3 label">
        <span>name</span>
        <span>value</span>
        <span className="text-right">status</span>
      </div>
      {secrets.map((s) => (
        <div
          key={s.name}
          className="grid grid-cols-[1.4fr_1.4fr_auto] items-center gap-4 border-b border-line px-5 py-3.5 text-[13px]"
        >
          <code className="text-cream">{s.name}</code>
          <span className={s.masked ? "text-muted" : "text-body"}>{s.value}</span>
          <span className="text-right">
            <Badge tone={written ? "blue" : "lime"}>{written ? "wrote" : "will write"}</Badge>
          </span>
        </div>
      ))}
      <div className="flex items-center justify-between px-5 py-3 text-[12.5px]">
        <span className="text-muted">
          {secrets.length} secrets · {written ? "written" : "will write"} to repo actions
        </span>
        <a
          href={account ? `https://github.com/${full}/settings/secrets/actions` : "#"}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-1.5 text-lime transition-colors hover:text-cream"
        >
          view in github <ArrowRight />
        </a>
      </div>
    </div>
  );
}
