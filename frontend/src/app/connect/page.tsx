import Link from "next/link";
import { Header } from "@/components/header";
import { ArrowRight, GithubMark, Shield } from "@/components/icons";
import { GITHUB_PERMISSIONS } from "@/lib/github";
import { getGithubConnection } from "@/lib/session";

export default async function ConnectPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const conn = await getGithubConnection();
  const { error } = await searchParams;
  const account = conn.connected ? conn.login : null;

  return (
    <div className="flex min-h-screen flex-col">
      <Header crumbs={[{ label: "connect" }]} container="max-w-3xl" />

      <main className="mx-auto flex w-full max-w-3xl flex-1 flex-col px-6 py-20">
        <div className="rise">
          <p className="label">
            step 01 / 02 · connect
          </p>
          <h1 className="mt-7 text-5xl font-extrabold leading-[1.05] tracking-tight text-cream">
            grant mountabo <span className="text-lime italic">just</span> what it needs.
          </h1>
          <p className="mt-6 max-w-xl text-[15px] leading-7 text-body">
            mountabo connects as a GitHub App through GitHub&apos;s authorization flow. it
            asks for the narrowest set of permissions that still let it add a deploy key,
            write one workflow file, and store the secrets that workflow reads. your token
            is exchanged on your machine and kept in your OS keychain: it never leaves.
          </p>
        </div>

        <div className="rise mt-12" style={{ animationDelay: "80ms" }}>
          <p className="label">permissions requested</p>
          <ul className="mt-4 divide-y divide-line overflow-hidden rounded-xl border border-line bg-surface">
            {GITHUB_PERMISSIONS.map((s) => (
              <li key={s.permission} className="flex flex-col gap-2 p-5 sm:flex-row sm:gap-6">
                <div className="flex w-48 shrink-0 items-center gap-3">
                  <code className="rounded-md border border-line-strong bg-surface-2 px-2 py-1 text-[13px] text-cream">
                    {s.permission}
                  </code>
                  <span className="text-[10px] text-lime">
                    {s.access}
                  </span>
                </div>
                <p className="text-[13.5px] leading-6 text-body">
                  <span className="text-cream">{s.label}</span>: {s.reason}
                </p>
              </li>
            ))}
          </ul>
        </div>

        <div className="rise mt-12 flex flex-col gap-4" style={{ animationDelay: "160ms" }}>
          {error && (
            <p className="rounded-lg border border-red-500/30 bg-red-500/5 px-4 py-3 text-[13px] text-red-300">
              {error === "state"
                ? "connection check failed (state mismatch). please try again."
                : error === "backend"
                  ? "couldn't reach the mountabo backend. is it running on localhost:7778?"
                  : error === "config"
                    ? "github isn't configured. set GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET in .env."
                    : "could not complete the github exchange. please try again."}
            </p>
          )}

          {account ? (
            <div className="flex items-center justify-between rounded-xl border border-blue/25 bg-blue/5 px-5 py-4">
              <span className="flex items-center gap-3 text-cream">
                <GithubMark className="text-blue" />
                connected as {account}
              </span>
              <Link
                href="/"
                className="flex items-center gap-2 text-[13px] text-lime transition-colors hover:text-cream"
              >
                continue <ArrowRight />
              </Link>
            </div>
          ) : (
            <a
              href="/api/github/authorize"
              className="cta-glow group flex items-center justify-between rounded-xl bg-lime-fill px-6 py-4 font-bold text-black transition-transform hover:-translate-y-0.5"
            >
              <span className="flex items-center gap-3">
                <GithubMark /> connect github
              </span>
              <span className="flex items-center gap-2 text-[13px] font-medium opacity-80">
                authorize on github.com <ArrowRight />
              </span>
            </a>
          )}

          <p className="flex items-center gap-2 text-[12px] text-muted">
            <Shield className="text-faint" />
            credentials never leave your machine. mountabo is not in the deploy path.
          </p>
        </div>
      </main>
    </div>
  );
}
