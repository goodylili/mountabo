"use client";

import { useEffect, useState } from "react";
import { GithubMark, Star } from "@/components/icons";

// mountabo's public repo. The header links here so users can go star it.
const REPO = "goodylili/mountabo";
const REPO_URL = `https://github.com/${REPO}`;

// RepoStar is the header's "star us on GitHub" badge: a link to the repo that
// shows the live star count when GitHub's public API answers (unauthenticated,
// best-effort, so a rate-limited or offline fetch just falls back to "star").
export function RepoStar() {
  const [stars, setStars] = useState<number | null>(null);

  useEffect(() => {
    const ctrl = new AbortController();
    fetch(`https://api.github.com/repos/${REPO}`, {
      signal: ctrl.signal,
      headers: { Accept: "application/vnd.github+json" },
    })
      .then((r) => (r.ok ? r.json() : null))
      .then((data: { stargazers_count?: number } | null) => {
        if (data && typeof data.stargazers_count === "number") setStars(data.stargazers_count);
      })
      .catch(() => {
        /* best-effort: leave the count off */
      });
    return () => ctrl.abort();
  }, []);

  return (
    <a
      href={REPO_URL}
      target="_blank"
      rel="noopener noreferrer"
      title={`star ${REPO} on GitHub`}
      aria-label={`star ${REPO} on GitHub`}
      className="hidden shrink-0 items-center gap-1.5 rounded-md border border-line bg-surface px-2.5 py-1 text-[12px] text-muted transition-colors hover:border-line-strong hover:text-cream sm:flex"
    >
      <GithubMark />
      <Star className="text-lime" />
      <span className="tabular-nums">{stars === null ? "star" : formatStars(stars)}</span>
    </a>
  );
}

function formatStars(n: number): string {
  return n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n);
}
