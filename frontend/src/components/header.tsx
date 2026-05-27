"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";
import { Book, GithubMark, LogoMark, Logout } from "@/components/icons";
import { RepoStar } from "@/components/repo-star";
import { ThemeToggle } from "@/components/theme-toggle";

export type Crumb = { label: string; href?: string; muted?: boolean };

// The header is intentionally identical on every view: a full-width top bar
// with the logo, the primary navigation, and the account/star/theme controls,
// at one consistent width regardless of the page (the `container` prop is kept
// only for compatibility with callers and no longer changes the bar). Anything
// that varies per page (the local connection status, the breadcrumb trail, the
// back affordance) lives in a separate controls row below the bar, so the header
// itself never shifts between views.
export function Header({
  crumbs,
  account,
  back,
}: {
  crumbs: Crumb[];
  account?: string | null;
  back?: boolean;
  /** Accepted for compatibility with existing callers; no longer used. */
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

  const showControls = back || crumbs.length > 0;

  return (
    <header className="shrink-0 border-b border-line">
      {/* firm top bar, identical across all views */}
      <div className="flex h-14 w-full items-center gap-3 px-4 text-[13px] sm:gap-5 sm:px-6 lg:px-8">
        <Link href="/" className="flex shrink-0 items-center gap-2 text-cream">
          <LogoMark className="text-lime" width={30} height={30} />
          <span className="text-lg font-bold tracking-tight">mountabo</span>
          <span className="hidden text-[13px] text-faint sm:inline">v0.4.2</span>
        </Link>

        <div className="ml-auto flex min-w-0 items-center gap-3 text-muted sm:gap-4">
          {account && (
            <>
              <nav className="hidden items-center gap-3 sm:flex">
                <Link
                  href="/"
                  className={pathname === "/" ? "text-cream" : "transition-colors hover:text-cream"}
                >
                  deploy
                </Link>
                <Link
                  href="/deployments"
                  className={
                    pathname?.startsWith("/deployments") ? "text-cream" : "transition-colors hover:text-cream"
                  }
                >
                  deployments
                </Link>
                <Link
                  href="/terminal"
                  className={
                    pathname?.startsWith("/terminal") ? "text-cream" : "transition-colors hover:text-cream"
                  }
                >
                  terminal
                </Link>
              </nav>
              <span className="hidden h-4 w-px bg-line sm:block" />
              <span className="flex min-w-0 items-center gap-2 text-cream">
                <GithubMark />
                <span className="truncate">{account}</span>
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
          )}
          <RepoStar />
          <span className="h-4 w-px bg-line" />
          <ThemeToggle />
        </div>
      </div>

      {/* controls row below the header: per-page status, breadcrumbs, and back */}
      {showControls && (
        <div className="flex h-11 w-full items-center gap-3 border-t border-line px-4 text-[13px] text-muted sm:gap-4 sm:px-6 lg:px-8">
          <span className="hidden items-center gap-2 rounded-md border border-line bg-surface px-2.5 py-1 md:flex">
            <span className="h-1.5 w-1.5 rounded-full bg-blue" />
            localhost:7777
          </span>

          {crumbs.length > 0 && (
            <nav className="flex items-center gap-2">
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
          )}

          {back && (
            <div className="ml-auto flex items-center gap-3">
              <Link href="/" className="flex items-center gap-1.5 transition-colors hover:text-cream">
                ‹ <span>back</span>
              </Link>
              <kbd className="rounded-md border border-line bg-surface px-2 py-1 text-[11px] text-muted">
                esc
              </kbd>
            </div>
          )}
        </div>
      )}
    </header>
  );
}
