"use client";

import { useMemo, useRef, useState } from "react";
import { ServerSelect } from "@/components/server-select";
import { ConfirmAction } from "@/components/confirm-action";
import {
  runCommand,
  suggestCommand,
  type AICommandResult,
  type ExecResult,
} from "@/lib/terminal";
import type { ServerView } from "@/lib/servers";

// One line in the console transcript: a command the operator ran (echoed back
// with a prompt) or the output and exit status it produced.
type Entry =
  | { kind: "command"; text: string }
  | { kind: "output"; text: string; exitCode: number; truncated: boolean }
  | { kind: "error"; text: string };

// TerminalConsole is the terminal page: pick a ready server, type a shell
// command, and see its combined output and exit status, exactly as a shell would
// show it. It also offers an AI helper that suggests a command for a plain
// English request. Critically, an AI suggestion is never run automatically: it
// is shown for review, and running it goes through the same confirm gate and the
// same exec path as anything typed by hand. The human stays in the loop for
// every command that touches the server.
export function TerminalConsole({ servers }: { servers: ServerView[] }) {
  // Only set-up (ready) servers can run commands; the backend enforces this too.
  const ready = useMemo(() => servers.filter((s) => s.status === "ready"), [servers]);

  const [serverId, setServerId] = useState<string | null>(ready[0]?.id ?? null);
  const [command, setCommand] = useState("");
  const [entries, setEntries] = useState<Entry[]>([]);
  const [running, setRunning] = useState(false);
  // The command queued behind the confirmation gate, or null when the gate is
  // closed. Nothing runs until the operator confirms here.
  const [pending, setPending] = useState<string | null>(null);

  const selected = ready.find((s) => s.id === serverId) ?? null;
  const transcriptRef = useRef<HTMLDivElement>(null);

  function scrollToEnd() {
    // After React paints the new entry, pin the transcript to the bottom.
    requestAnimationFrame(() => {
      const el = transcriptRef.current;
      if (el) el.scrollTop = el.scrollHeight;
    });
  }

  // ask opens the confirmation gate for the current command. Nothing is sent yet.
  function ask() {
    const trimmed = command.trim();
    if (!trimmed || !serverId || running) return;
    setPending(trimmed);
  }

  // confirmRun is the only path that actually sends a command to the server,
  // reached only after the operator confirms in the gate.
  async function confirmRun() {
    const cmd = pending;
    setPending(null);
    if (!cmd || !serverId) return;

    setRunning(true);
    setEntries((prev) => [...prev, { kind: "command", text: cmd }]);
    setCommand("");
    scrollToEnd();
    try {
      const result: ExecResult = await runCommand(serverId, cmd);
      setEntries((prev) => [
        ...prev,
        {
          kind: "output",
          text: result.output,
          exitCode: result.exitCode,
          truncated: result.truncated,
        },
      ]);
    } catch (err) {
      setEntries((prev) => [
        ...prev,
        { kind: "error", text: err instanceof Error ? err.message : "the command could not be run" },
      ]);
    } finally {
      setRunning(false);
      scrollToEnd();
    }
  }

  return (
    <main className="mx-auto w-full max-w-[1100px] flex-1 px-4 py-10 sm:px-6 lg:px-8">
      <p className="label">terminal</p>
      <h1 className="mt-2 text-2xl font-bold tracking-tight text-cream">
        run a command on your server
      </h1>
      <p className="mt-2 max-w-2xl text-[13px] leading-6 text-body">
        Pick one of your set-up servers, type a shell command, and see its output and exit status.
        Commands run as the mountabo user over SSH. Every command is confirmed before it runs.
      </p>

      <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:items-center">
        <ServerSelect servers={ready} value={serverId} onChange={setServerId} />
        {selected && (
          <span className="text-[12px] text-muted">
            connected as <span className="text-cream">mountabo</span> on {selected.ip}
          </span>
        )}
      </div>

      <CommandConsole
        entries={entries}
        running={running}
        transcriptRef={transcriptRef}
        command={command}
        onCommandChange={setCommand}
        onSubmit={ask}
        disabled={!serverId}
      />

      <AIHelper serverContext={contextFor(selected)} onUse={setCommand} />

      {pending !== null && (
        <ConfirmAction
          title="run this command?"
          subtitle={selected ? `${selected.name} · ${selected.ip}` : undefined}
          summary={
            <>
              This will run the command below on the server as the mountabo user over SSH. Review it
              before confirming: only you decide what runs.
            </>
          }
          steps={pending}
          stepsLabel="exact command that will run on the server"
          confirmLabel="run command"
          onConfirm={confirmRun}
          onCancel={() => setPending(null)}
        />
      )}
    </main>
  );
}

// CommandConsole renders the transcript and the command input. It is a dark
// monospace console: each command is echoed with a prompt, each result is shown
// with its exit status tinted green (0) or red (non zero).
function CommandConsole({
  entries,
  running,
  transcriptRef,
  command,
  onCommandChange,
  onSubmit,
  disabled,
}: {
  entries: Entry[];
  running: boolean;
  transcriptRef: React.RefObject<HTMLDivElement | null>;
  command: string;
  onCommandChange: (v: string) => void;
  onSubmit: () => void;
  disabled: boolean;
}) {
  return (
    <div className="mt-5 overflow-hidden rounded-xl border border-line bg-black/40">
      <div
        ref={transcriptRef}
        className="h-[22rem] overflow-y-auto overscroll-contain px-4 py-3 font-mono text-[12.5px] leading-6"
      >
        {entries.length === 0 ? (
          <p className="text-faint">the output of the commands you run will appear here.</p>
        ) : (
          <div className="space-y-2">
            {entries.map((entry, i) => (
              <TranscriptEntry key={i} entry={entry} />
            ))}
            {running && <p className="text-muted">running…</p>}
          </div>
        )}
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          onSubmit();
        }}
        className="flex items-center gap-2 border-t border-line bg-surface px-3 py-2.5"
      >
        <span className="shrink-0 font-mono text-[13px] text-lime">$</span>
        <input
          value={command}
          onChange={(e) => onCommandChange(e.target.value)}
          disabled={disabled || running}
          placeholder={disabled ? "select a ready server first" : "type a command, e.g. df -h"}
          spellCheck={false}
          autoCapitalize="off"
          autoCorrect="off"
          className="min-w-0 flex-1 bg-transparent font-mono text-[13px] text-cream placeholder:text-faint focus:outline-none disabled:opacity-60"
        />
        <button
          type="submit"
          disabled={disabled || running || command.trim() === ""}
          className="shrink-0 rounded-md border border-lime/50 bg-lime/[0.08] px-4 py-1.5 text-[12px] font-medium text-lime transition-colors hover:bg-lime/[0.16] disabled:cursor-not-allowed disabled:opacity-40"
        >
          run
        </button>
      </form>
    </div>
  );
}

function TranscriptEntry({ entry }: { entry: Entry }) {
  if (entry.kind === "command") {
    return (
      <div className="flex gap-2 break-words">
        <span className="shrink-0 text-lime">$</span>
        <span className="text-cream">{entry.text}</span>
      </div>
    );
  }
  if (entry.kind === "error") {
    return <pre className="whitespace-pre-wrap break-words text-red-300">{entry.text}</pre>;
  }
  const ok = entry.exitCode === 0;
  return (
    <div>
      {entry.text !== "" && (
        <pre className="whitespace-pre-wrap break-words text-body">{entry.text}</pre>
      )}
      <p className={`mt-0.5 text-[11px] ${ok ? "text-lime" : "text-red-300"}`}>
        exit {entry.exitCode}
        {entry.truncated && <span className="text-faint"> · output truncated</span>}
      </p>
    </div>
  );
}

// AIHelper lets the operator describe what they want in plain English and get a
// suggested command back with a short explanation. The suggestion is filled into
// the command input for review; it is never run on its own. When the backend has
// no Anthropic key it returns configured=false and we show the hint to set
// ANTHROPIC_API_KEY rather than an error.
function AIHelper({
  serverContext,
  onUse,
}: {
  serverContext: string;
  onUse: (command: string) => void;
}) {
  const [prompt, setPrompt] = useState("");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<AICommandResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function ask() {
    const trimmed = prompt.trim();
    if (!trimmed || loading) return;
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const res = await suggestCommand(trimmed, serverContext);
      setResult(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not get a suggestion");
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="mt-6 rounded-xl border border-line bg-surface p-4 sm:p-5">
      <p className="label">ask the AI helper</p>
      <p className="mt-1 text-[13px] leading-6 text-body">
        Describe what you want to do and the helper suggests a command. It only suggests: you review
        it, then run it from the console above. Nothing runs on its own.
      </p>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          ask();
        }}
        className="mt-3 flex flex-col gap-2 sm:flex-row"
      >
        <input
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          disabled={loading}
          placeholder="e.g. show the largest files in /var/log"
          className="min-w-0 flex-1 rounded-md border border-line bg-bg px-3 py-2.5 text-[13px] text-cream placeholder:text-faint transition-colors hover:border-line-strong focus:border-line-strong focus:outline-none disabled:opacity-60"
        />
        <button
          type="submit"
          disabled={loading || prompt.trim() === ""}
          className="shrink-0 rounded-md border border-line bg-surface-2 px-4 py-2.5 text-[12px] font-medium text-cream transition-colors hover:border-line-strong disabled:cursor-not-allowed disabled:opacity-40 sm:py-2"
        >
          {loading ? "thinking…" : "suggest"}
        </button>
      </form>

      {error && <p className="mt-3 text-[12px] text-red-300">{error}</p>}

      {result && !result.configured && (
        <div className="mt-3 rounded-lg border border-line bg-bg px-4 py-3 text-[12px] leading-6 text-muted">
          {result.explanation}
        </div>
      )}

      {result && result.configured && (
        <div className="mt-3 rounded-lg border border-line bg-bg p-4">
          {result.command !== "" ? (
            <>
              <p className="label">suggested command</p>
              <pre className="mt-1.5 overflow-x-auto whitespace-pre-wrap break-words font-mono text-[12.5px] leading-6 text-cream">
                {result.command}
              </pre>
              {result.explanation !== "" && (
                <p className="mt-2 text-[12px] leading-6 text-body">{result.explanation}</p>
              )}
              <button
                onClick={() => onUse(result.command)}
                className="mt-3 rounded-md border border-lime/50 bg-lime/[0.08] px-4 py-1.5 text-[12px] font-medium text-lime transition-colors hover:bg-lime/[0.16]"
              >
                use this command
              </button>
              <p className="mt-2 text-[11px] text-faint">
                this fills the console above so you can review it, then confirm before it runs.
              </p>
            </>
          ) : (
            <p className="text-[12px] leading-6 text-muted">
              {result.explanation || "the helper could not suggest a command for that request."}
            </p>
          )}
        </div>
      )}
    </section>
  );
}

// contextFor builds a short server-context hint for the AI helper from what the
// frontend already knows, so suggestions can be tailored (e.g. the OS), without
// running anything on the box.
function contextFor(server: ServerView | null): string {
  if (!server) return "";
  const os = server.specs.os || "unknown OS";
  return `Target server OS: ${os}. Commands run as the non-root sudo user "mountabo".`;
}
