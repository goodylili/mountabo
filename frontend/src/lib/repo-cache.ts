"use client";

import type { Source } from "@/lib/data";

// Listing repositories is the slowest call in the app (full GitHub pagination
// plus per-repo container detection), yet the list barely changes between
// visits. So the deploy picker caches it per account in the browser with a 12
// hour TTL: a returning user sees their repositories instantly, and the refresh
// button forces a refetch for when they have created a repository or been
// granted access to a new one.
const TTL_MS = 12 * 60 * 60 * 1000;
const PREFIX = "mountabo:repos:";

type Cached = { at: number; repos: Source[] };

function key(account: string): string {
  return `${PREFIX}${account}`;
}

// Reads cached repositories for an account, or null when they are absent,
// expired, or unreadable (so the caller falls back to a fresh fetch).
export function readCachedRepos(account: string): Cached | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(key(account));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Cached;
    if (typeof parsed?.at !== "number" || !Array.isArray(parsed.repos)) return null;
    if (Date.now() - parsed.at > TTL_MS) return null;
    return parsed;
  } catch {
    return null;
  }
}

// Stores repositories for an account and returns the timestamp written.
export function writeCachedRepos(account: string, repos: Source[]): number {
  const at = Date.now();
  if (typeof window === "undefined") return at;
  try {
    window.localStorage.setItem(key(account), JSON.stringify({ at, repos } satisfies Cached));
  } catch {
    // Storage disabled or over quota: skip caching, the picker still works.
  }
  return at;
}

// Clears cached repositories. With an account it removes just that entry; with
// no argument it purges every mountabo repo cache entry, so a stale or wrong
// listing for any account is forgotten and the next load fetches fresh.
export function clearCachedRepos(account?: string): void {
  if (typeof window === "undefined") return;
  try {
    if (account) {
      window.localStorage.removeItem(key(account));
      return;
    }
    for (let i = window.localStorage.length - 1; i >= 0; i--) {
      const k = window.localStorage.key(i);
      if (k?.startsWith(PREFIX)) window.localStorage.removeItem(k);
    }
  } catch {
    // Storage disabled: there is nothing to clear.
  }
}

// Fetches the connected account's repositories through the local proxy route.
export async function fetchRepos(): Promise<Source[]> {
  const resp = await fetch("/api/repos", { cache: "no-store" });
  if (!resp.ok) throw new Error(`repos request failed: ${resp.status}`);
  return (await resp.json()) as Source[];
}
