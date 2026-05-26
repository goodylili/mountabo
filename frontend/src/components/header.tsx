"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";
import { Book, GithubMark, LogoMark, Logout } from "@/components/icons";
import { ThemeToggle } from "@/components/theme-toggle";

export type Crumb = { label: string; href?: string; muted?: boolean };

export function Header({
  crumbs,
  account,
  back,
  container = "max-w-6xl",
}: {
  crumbs: Crumb[];
  account?: string | null;
  back?: boolean;
  container?: string;
}) {
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (!back) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") router.push("/");
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [back, router]);

  return (
    <header className="shrink-0 border-b border-line">
      <div
        className={`mx-auto flex h-14 w-full items-center gap-5 px-6 text-[13px] ${container}`}
      >
      <Link href="/" className="flex items-center gap-2 text-cream">
        <LogoMark className="text-lime" width={30} height={30} />
        <span className="text-lg font-bold tracking-tight">mountabo</span>
        <span className="text-[13px] text-faint">v0.4.2</span>
      </Link>

      <span className="text-faint">|</span>

      <span className="flex items-center gap-2 rounded-md border border-line bg-surface px-2.5 py-1 text-muted">
        <span className="h-1.5 w-1.5 rounded-full bg-blue" />
        localhost:7777
      </span>

      <nav className="flex items-center gap-2 text-muted">
        {crumbs.map((c, i) => (
          <span key={c.label} className="flex items-center gap-2">
            {i > 0 && <span className="text-faint">›</span>}
            {c.href ? (
              <Link href={c.href} className="transition-colors hover:text-cream">
                {c.label}
              </Link>
            ) : (
              <span className={c.muted ? "text-muted" : "text-cream"}>{c.label}</span>
            )}
          </span>
        ))}
      </nav>

      <div className="ml-auto flex items-center gap-4 text-muted">
        {back ? (
          <>
            <Link
              href="/"
              className="flex items-center gap-1.5 transition-colors hover:text-cream"
            >
              ‹ <span>back</span>
            </Link>
            <kbd className="rounded-md border border-line bg-surface px-2 py-1 text-[11px] text-muted">
              esc
            </kbd>
          </>
        ) : account ? (
          <>
            <nav className="mr-1 flex items-center gap-3">
              <Link
                href="/"
                className={pathname === "/" ? "text-cream" : "transition-colors hover:text-cream"}
              >
                deploy
              </Link>
              <Link
                href="/monitor"
                className={
                  pathname?.startsWith("/monitor") ? "text-cream" : "transition-colors hover:text-cream"
                }
              >
                monitor
              </Link>
            </nav>
            <span className="h-4 w-px bg-line" />
            <span className="flex items-center gap-2 text-cream">
              <GithubMark />
              {account}
            </span>
            <button className="transition-colors hover:text-cream" aria-label="docs">
              <Book />
            </button>
            <a
              href="/api/github/disconnect"
              className="transition-colors hover:text-cream"
              aria-label="disconnect"
            >
              <Logout />
            </a>
          </>
        ) : (
          <Link
            href="/connect"
            className="flex items-center gap-2 rounded-md border border-lime/40 px-3 py-1.5 text-lime transition-colors hover:bg-lime/10"
          >
            <GithubMark /> connect github
          </Link>
        )}
        <span className="h-4 w-px bg-line" />
        <ThemeToggle />
      </div>
      </div>
    </header>
  );
}
