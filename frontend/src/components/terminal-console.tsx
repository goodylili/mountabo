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
  | { kind: "command"; text: string; promptHost: string; promptPath: string }
  | { kind: "output"; text: string; exitCode: number; truncated: boolean }
  | { kind: "error"; text: string };

// prettyCwd shortens an absolute path under the mountabo user's home to ~/...
// so the prompt reads like a tuned shell instead of a verbose absolute path.
function prettyCwd(p: string): string {
  if (!p) return "~";
  if (p === "/home/mountabo") return "~";
  if (p.startsWith("/home/mountabo/")) return "~" + p.slice("/home/mountabo".length);
  return p;
}

// TerminalConsole is the terminal page, dressed up to look and feel like the
// macOS Terminal app: a window with three traffic light dots and a centered
// title, a near black body, and an inline shell prompt at the bottom. Pick a
// ready server, type a shell command, and see its combined output and exit
// status, exactly as a shell would show it. It also offers an AI helper that
// suggests a command for a plain English request. Critically, an AI suggestion
// is never run automatically: it is shown for review, and running it goes
// through the same confirm gate and the same exec path as anything typed by
// hand. The human stays in the loop for every command that touches the server.
export function TerminalConsole({ servers }: { servers: ServerView[] }) {
  // Only set-up (ready) servers can run commands; the backend enforces this too.
  const ready = useMemo(() => servers.filter((s) => s.status === "ready"), [servers]);

  const [serverId, setServerId] = useState<string | null>(ready[0]?.id ?? null);
  const [command, setCommand] = useState("");
  const [entries, setEntries] = useState<Entry[]>([]);
  const [running, setRunning] = useState(false);
  // Working directory the shell ended in after the last command, threaded back
  // to the next call so `cd` and relative paths feel persistent across
  // separate SSH sessions. Empty means start in the user's home.
  const [cwd, setCwd] = useState<string>("");
  // The command queued behind the confirmation gate, or null when the gate is
  // closed. Nothing runs until the operator confirms here.
  const [pending, setPending] = useState<string | null>(null);
  // Express mode runs commands immediately, skipping the per-command confirm.
  // Off by default, so the safe path (confirm each command) is the default.
  const [express, setExpress] = useState(false);

  const selected = ready.find((s) => s.id === serverId) ?? null;
  // The host token shown in the prompt, e.g. "mountabo@web1". When no server is
  // selected we still show a sensible placeholder rather than a blank prompt.
  const host = selected ? `mountabo@${selected.name}` : "mountabo@no-server";
  const transcriptRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  function scrollToEnd() {
    // After React paints the new entry, pin the transcript to the bottom.
    requestAnimationFrame(() => {
      const el = transcriptRef.current;
      if (el) el.scrollTop = el.scrollHeight;
    });
  }

  // ask runs the current command. In express mode it executes immediately; with
  // express off it opens the confirmation gate first and nothing is sent yet.
  function ask() {
    const trimmed = command.trim();
    if (!trimmed || !serverId || running) return;
    if (express) {
      void execute(trimmed);
      return;
    }
    setPending(trimmed);
  }

  // confirmRun runs the gated command after the operator confirms.
  function confirmRun() {
    const cmd = pending;
    setPending(null);
    void execute(cmd ?? "");
  }

  // execute is the only path that actually sends a command to the server,
  // reached from the confirm gate or directly in express mode.
  async function execute(cmd: string) {
    if (!cmd || !serverId) return;
    setRunning(true);
    const pathAtRun = prettyCwd(cwd);
    setEntries((prev) => [
      ...prev,
      { kind: "command", text: cmd, promptHost: host, promptPath: pathAtRun },
    ]);
    setCommand("");
    scrollToEnd();
    try {
      const result: ExecResult = await runCommand(serverId, cmd, cwd);
      if (result.cwd) setCwd(result.cwd);
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

  // The window title mirrors the macOS Terminal tab title for an ssh session.
  const windowTitle = selected ? `${host}  ~  ssh` : "mountabo  ~  no server selected";

  return (
    <main className="mx-auto w-full max-w-[1100px] flex-1 px-4 pb-16 pt-10 sm:px-6 sm:pt-16 lg:px-8">
      <p className="label">terminal</p>
      <h1 className="mt-6 text-4xl font-extrabold leading-[1.02] tracking-tight text-cream sm:text-5xl sm:leading-[0.98] lg:text-6xl">
        a shell on your server,
        <br />
        right in the <span className="italic text-lime">browser.</span>
      </h1>
      <p className="mt-6 max-w-2xl text-[16px] leading-8 text-body">
        pick a server, run shell commands over ssh, and read the output here. ask the helper for a
        command and it suggests one you review and run, never on its own. everything runs as the
        mountabo user, confirmed before it runs unless you turn on express mode.
      </p>

      <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:items-center">
        <ServerSelect
          servers={ready}
          value={serverId}
          onChange={(id) => {
            setServerId(id);
            // Switching servers should not carry the old server's cwd over,
            // since the path may not exist (or mean something different) there.
            setCwd("");
          }}
        />
        {selected && (
          <span className="text-[12px] text-muted">
            connected as <span className="text-cream">mountabo</span> on {selected.ip}
          </span>
        )}
        <button
          type="button"
          onClick={() => setExpress((v) => !v)}
          aria-pressed={express}
          title="Express mode runs commands immediately, without confirming each one."
          className={`flex items-center gap-2 rounded-md border px-3 py-1.5 text-[12px] font-medium transition-colors sm:ml-auto ${
            express
              ? "border-lime/50 bg-lime/[0.08] text-lime"
              : "border-line text-muted hover:border-line-strong hover:text-cream"
          }`}
        >
          <span className={`h-1.5 w-1.5 rounded-full ${express ? "bg-lime" : "bg-muted"}`} />
          express mode {express ? "on" : "off"}
        </button>
      </div>

      <TerminalWindow
        title={windowTitle}
        entries={entries}
        running={running}
        transcriptRef={transcriptRef}
        inputRef={inputRef}
        host={host}
        path={prettyCwd(cwd)}
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

// TerminalWindow renders the console as a macOS style terminal window: a title
// bar with three traffic light dots and a centered title, a near black body
// holding the scrollable transcript, and an inline shell prompt at the bottom
// that reads like a real shell. Each command is echoed with its prompt, each
// result is shown with its exit status tinted green (0) or red (non zero).
function TerminalWindow({
  title,
  entries,
  running,
  transcriptRef,
  inputRef,
  host,
  path,
  command,
  onCommandChange,
  onSubmit,
  disabled,
}: {
  title: string;
  entries: Entry[];
  running: boolean;
  transcriptRef: React.RefObject<HTMLDivElement | null>;
  inputRef: React.RefObject<HTMLInputElement | null>;
  host: string;
  path: string;
  command: string;
  onCommandChange: (v: string) => void;
  onSubmit: () => void;
  disabled: boolean;
}) {
  // Clicking anywhere in the body focuses the prompt, like a real terminal.
  function focusPrompt() {
    inputRef.current?.focus();
  }

  return (
    <div className="mt-5 overflow-hidden rounded-xl border border-line-strong bg-[#000000] shadow-2xl">
      {/* title bar: traffic light dots on the left, centered ssh title */}
      <div className="relative flex h-9 items-center border-b border-line bg-surface-2 px-4">
        <div className="flex items-center gap-2" aria-hidden>
          <span className="h-3 w-3 rounded-full bg-[#ff5f57]" />
          <span className="h-3 w-3 rounded-full bg-[#febc2e]" />
          <span className="h-3 w-3 rounded-full bg-[#28c840]" />
        </div>
        <span className="pointer-events-none absolute inset-x-0 mx-auto truncate px-24 text-center font-mono text-[12px] text-muted">
          {title}
        </span>
      </div>

      {/* body: near black panel holding the transcript and the inline prompt */}
      <div
        ref={transcriptRef}
        onClick={focusPrompt}
        className="h-[28rem] cursor-text overflow-y-auto overscroll-contain bg-[#000000] px-5 py-4 font-mono text-[13.5px] leading-7 text-body"
      >
        {entries.length === 0 && (
          <p className="text-faint">
            {disabled
              ? "select a ready server above to open a session."
              : "the output of the commands you run will appear here."}
          </p>
        )}

        {entries.map((entry, i) => (
          <TranscriptEntry key={i} entry={entry} />
        ))}

        {running && <p className="text-muted">running…</p>}

        {/* inline prompt line: host, path, sigil, then the live input */}
        {!running && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              onSubmit();
            }}
            className="flex items-baseline gap-2"
          >
            <PromptSigil host={host} path={path} />
            <input
              ref={inputRef}
              value={command}
              onChange={(e) => onCommandChange(e.target.value)}
              disabled={disabled}
              placeholder={disabled ? "select a ready server first" : ""}
              spellCheck={false}
              autoCapitalize="off"
              autoCorrect="off"
              autoComplete="off"
              aria-label="shell command"
              className="min-w-0 flex-1 bg-transparent font-mono text-[13.5px] text-cream caret-lime placeholder:text-faint focus:outline-none disabled:opacity-60"
            />
            {/* a blinking block caret shows the prompt is live when empty */}
            {!disabled && command === "" && (
              <span className="-ml-1 inline-block h-[1.05em] w-[0.55em] animate-pulse self-center bg-lime/80" aria-hidden />
            )}
          </form>
        )}
      </div>
    </div>
  );
}

// PromptSigil renders the shell prompt prefix, e.g. "mountabo@web1 ~/apps %",
// with the host tinted lime, the path blue, and the trailing % muted, the way
// a tuned shell prompt reads. Path defaults to ~ so the prompt is never blank.
function PromptSigil({ host, path }: { host: string; path?: string }) {
  return (
    <span className="shrink-0 select-none font-mono text-[13.5px]">
      <span className="text-lime">{host}</span>
      <span className="text-blue"> {path && path !== "" ? path : "~"} </span>
      <span className="text-muted">%</span>
    </span>
  );
}

function TranscriptEntry({ entry }: { entry: Entry }) {
  if (entry.kind === "command") {
    return (
      <div className="flex items-baseline gap-2 break-words">
        <PromptSigil host={entry.promptHost} path={entry.promptPath} />
        <span className="text-cream">{entry.text}</span>
      </div>
    );
  }
  if (entry.kind === "error") {
    return <pre className="mt-1 whitespace-pre-wrap break-words text-red-300">{entry.text}</pre>;
  }
  const ok = entry.exitCode === 0;
  return (
    <div className="mb-3 mt-1">
      {entry.text !== "" && (
        <pre className="whitespace-pre-wrap break-words text-body">{entry.text}</pre>
      )}
      <p className={`mt-0.5 text-[12px] ${ok ? "text-lime" : "text-red-300"}`}>
        exit {entry.exitCode}
        {entry.truncated && <span className="text-faint"> · output truncated</span>}
      </p>
    </div>
  );
}

// AIHelper lets the operator describe what they want in plain English and get a
// suggested command back with a short explanation. It is a collapsible panel
// below the terminal window. The suggestion is filled into the command input for
// review; it is never run on its own. When the backend has no Anthropic key it
// returns configured=false and we show the hint to set ANTHROPIC_API_KEY rather
// than an error.
function AIHelper({
  serverContext,
  onUse,
}: {
  serverContext: string;
  onUse: (command: string) => void;
}) {
  const [open, setOpen] = useState(false);
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
    <section className="mt-6 rounded-xl border border-line bg-surface">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left sm:px-5"
      >
        <span>
          <span className="label">ask the AI helper</span>
          <span className="mt-1 block text-[13px] leading-6 text-body">
            Describe what you want to do and the helper suggests a command. It only suggests: you
            review it, then run it from the terminal above. Nothing runs on its own.
          </span>
        </span>
        <span className="shrink-0 text-[12px] text-muted">{open ? "hide" : "show"}</span>
      </button>

      {open && (
        <div className="border-t border-line px-4 py-4 sm:px-5">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              ask();
            }}
            className="flex flex-col gap-2 sm:flex-row"
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
                    this fills the terminal above so you can review it, then run it.
                  </p>
                </>
              ) : (
                <p className="text-[12px] leading-6 text-muted">
                  {result.explanation || "the helper could not suggest a command for that request."}
                </p>
              )}
            </div>
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
