"use client";

import { useState } from "react";
import { ChevronDown, Plus } from "@/components/icons";
import type { Domain, ServerView } from "@/lib/servers";

// DomainFormValue is what the add-domain form hands the parent; aliases (e.g. the
// www variant) are derived inside the form before it leaves.
export type DomainFormValue = {
  host: string;
  aliases: string[];
  upstream: string;
  email: string;
  staging: boolean;
};

const DEFAULT_PORT = "3000";

// A plausible FQDN: at least one dot, valid label characters. Mirrors the
// backend's check so the button only lights up for inputs the server accepts.
const FQDN = /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/;

// ServerDomains is the per-server custom-domain panel shown under a selected,
// ready server, a sibling of ServerOptions. It lists the domains already fronted
// by nginx + HTTPS and offers a one-field add: type a domain, pick the app port,
// done. Adding/removing hands off to the parent, which streams the live log.
export function ServerDomains({
  server,
  onAdd,
  onRemove,
}: {
  server: ServerView;
  onAdd: (value: DomainFormValue) => void;
  onRemove: (host: string) => void;
}) {
  const domains = server.domains ?? [];
  const [host, setHost] = useState("");
  const [port, setPort] = useState(DEFAULT_PORT);
  const [www, setWww] = useState(true);
  const [email, setEmail] = useState("");
  const [staging, setStaging] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);

  // Forgive a pasted URL (strip scheme + path) and normalise case.
  const cleanHost = host
    .trim()
    .toLowerCase()
    .replace(/^https?:\/\//, "")
    .replace(/\/.*$/, "");
  const validHost = FQDN.test(cleanHost);
  const portNum = Number(port);
  const validPort = port === "" || (Number.isInteger(portNum) && portNum >= 1 && portNum <= 65535);
  // "also serve www" only makes sense for an apex domain (not already a
  // subdomain like www. or api.).
  const wwwApplies = validHost && cleanHost.split(".").length === 2;
  const canAdd = validHost && validPort;

  function add() {
    if (!canAdd) return;
    onAdd({
      host: cleanHost,
      aliases: www && wwwApplies ? [`www.${cleanHost}`] : [],
      upstream: port.trim() || DEFAULT_PORT,
      email: email.trim(),
      staging,
    });
    setHost("");
    setPort(DEFAULT_PORT);
    setEmail("");
  }

  return (
    <div className="border-t border-line px-4 py-5">
      <p className="text-[13px] leading-6 text-body">
        domains: point a custom domain at this server. mountabo installs nginx, fronts your app on{" "}
        <span className="text-cream">https</span> with a free Let&apos;s Encrypt certificate, and keeps it
        renewed.
      </p>

      {domains.length > 0 && (
        <ul className="mt-4 space-y-2">
          {domains.map((d) => (
            <DomainRow key={d.host} domain={d} onRemove={() => onRemove(d.host)} />
          ))}
        </ul>
      )}

      <div className="mt-4 rounded-lg border border-line bg-surface-2 p-4">
        <label className="block">
          <span className="mb-1.5 block text-[12px] text-muted">domain</span>
          <input
            value={host}
            onChange={(e) => setHost(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && add()}
            placeholder="app.example.com"
            className={`w-full rounded-md border bg-surface px-3 py-2.5 text-[14px] text-cream placeholder:text-muted focus:outline-none ${
              !host || validHost ? "border-line focus:border-line-strong" : "border-red-500/40"
            }`}
          />
        </label>

        <div className="mt-3 grid grid-cols-2 gap-3">
          <label className="block">
            <span className="mb-1.5 block text-[12px] text-muted">app port</span>
            <input
              value={port}
              onChange={(e) => setPort(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && add()}
              placeholder={DEFAULT_PORT}
              inputMode="numeric"
              className={`w-full rounded-md border bg-surface px-3 py-2.5 text-[14px] text-cream placeholder:text-muted focus:outline-none ${
                validPort ? "border-line focus:border-line-strong" : "border-red-500/40"
              }`}
            />
          </label>
          <label className={`flex items-end pb-2 ${wwwApplies ? "" : "opacity-40"}`}>
            <span className="flex cursor-pointer items-center gap-2 text-[13px] text-body">
              <input
                type="checkbox"
                checked={www && wwwApplies}
                disabled={!wwwApplies}
                onChange={(e) => setWww(e.target.checked)}
                className="h-4 w-4 accent-lime"
              />
              also serve www.{cleanHost || "…"}
            </span>
          </label>
        </div>

        <button
          onClick={() => setShowAdvanced((v) => !v)}
          aria-expanded={showAdvanced}
          className="mt-3 flex items-center gap-1.5 text-[12px] text-muted transition-colors hover:text-cream"
        >
          <ChevronDown className={`text-faint transition-transform ${showAdvanced ? "" : "-rotate-90"}`} />
          advanced
        </button>
        {showAdvanced && (
          <div className="mt-3 space-y-3">
            <label className="block">
              <span className="mb-1.5 block text-[12px] text-muted">
                contact email (Let&apos;s Encrypt expiry notices, optional)
              </span>
              <input
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
                className="w-full rounded-md border border-line bg-surface px-3 py-2.5 text-[14px] text-cream placeholder:text-muted focus:border-line-strong focus:outline-none"
              />
            </label>
            <label className="flex cursor-pointer items-center gap-2 text-[13px] text-body">
              <input
                type="checkbox"
                checked={staging}
                onChange={(e) => setStaging(e.target.checked)}
                className="h-4 w-4 accent-lime"
              />
              use Let&apos;s Encrypt staging (untrusted cert, for testing DNS without rate limits)
            </label>
          </div>
        )}

        <button
          onClick={add}
          disabled={!canAdd}
          className="mt-4 flex w-full items-center justify-center gap-2 rounded-lg border border-lime/50 bg-lime/[0.06] px-4 py-2.5 text-[12.5px] font-medium text-lime transition-colors hover:bg-lime/[0.12] disabled:cursor-not-allowed disabled:opacity-40"
        >
          <Plus /> {!host ? "add a domain" : validHost ? `add ${cleanHost}` : "enter a valid domain"}
        </button>

        <p className="mt-3 text-[12px] leading-6 text-muted">
          point your domain&apos;s DNS here first: an <span className="text-cream">A record</span> for{" "}
          <span className="text-cream">{cleanHost || "your domain"}</span> →{" "}
          <span className="text-cream">{server.ip}</span>. the certificate is verified over http, so the
          domain must already resolve to this server.
        </p>
      </div>
    </div>
  );
}

function DomainRow({ domain, onRemove }: { domain: Domain; onRemove: () => void }) {
  const aliases = domain.aliases ?? [];
  return (
    <li className="flex items-center gap-3 rounded-lg border border-line bg-surface-2 px-4 py-3">
      <span className="min-w-0 flex-1">
        <a
          href={`https://${domain.host}`}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-2 text-[14px] font-medium text-cream transition-colors hover:text-lime hover:underline"
        >
          {domain.host}
        </a>
        <span className="mt-1 flex flex-wrap items-center gap-2 text-[12px] text-muted">
          <span className="text-lime">https</span>
          <span className="text-faint">·</span>
          <span>→ localhost:{domain.upstream}</span>
          {aliases.map((a) => (
            <span key={a} className="rounded border border-line px-1.5 py-0.5 text-[11px]">
              + {a}
            </span>
          ))}
        </span>
      </span>
      <button
        onClick={onRemove}
        className="shrink-0 rounded-md border border-line px-3 py-1.5 text-[12px] text-muted transition-colors hover:border-red-400/50 hover:text-red-300"
      >
        remove
      </button>
    </li>
  );
}
