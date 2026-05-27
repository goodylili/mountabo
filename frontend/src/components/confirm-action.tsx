"use client";

import { useEffect, type ReactNode } from "react";

// ConfirmAction is the single confirmation gate for everything mountabo runs on
// a server. Nothing touches a server until the operator confirms here. It shows
// a plain-English summary of what will happen and the exact steps that will run
// (a preformatted script/command string, or a list of step strings) in a
// scrollable monospace block, modelled on the StreamLog overlay. Confirm fires
// onConfirm; Cancel (or the backdrop, or Escape) closes via onCancel.
export function ConfirmAction({
  title,
  subtitle,
  summary,
  steps,
  stepsLabel = "exact steps that will run on the server",
  confirmLabel = "confirm",
  cancelLabel = "cancel",
  loading = false,
  loadingHint = "loading the exact steps…",
  destructive = false,
  onConfirm,
  onCancel,
}: {
  title: string;
  subtitle?: string;
  // A short plain-English explanation of what confirming does.
  summary: ReactNode;
  // The exact commands/config: either one preformatted block (e.g. a script) or
  // a list of discrete step strings. While loading, leave undefined.
  steps?: string | string[];
  stepsLabel?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  // While true the steps are still being fetched; confirm is disabled.
  loading?: boolean;
  loadingHint?: string;
  // Tints the confirm button red for irreversible actions (delete, remove).
  destructive?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  // Escape cancels, so the overlay is dismissable by keyboard like a dialog.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCancel();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onCancel]);

  const stepLines = Array.isArray(steps) ? steps : undefined;
  const stepBlock = typeof steps === "string" ? steps : undefined;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-3 sm:p-6"
      onClick={onCancel}
    >
      <div
        role="dialog"
        aria-modal="true"
        onClick={(e) => e.stopPropagation()}
        className="flex max-h-[85vh] w-full max-w-2xl flex-col overflow-hidden rounded-xl border border-line bg-surface sm:max-h-[80vh]"
      >
        <div className="flex items-center justify-between gap-3 border-b border-line px-4 py-3 sm:px-5">
          <span className="flex min-w-0 items-center gap-2 text-[13px] text-cream">
            <span className="truncate">{title}</span>
            {subtitle && (
              <>
                <span className="text-faint">·</span>
                <span className="truncate text-muted">{subtitle}</span>
              </>
            )}
          </span>
          <button
            onClick={onCancel}
            className="shrink-0 rounded-md border border-line px-2.5 py-1 text-[12px] text-muted transition-colors hover:text-cream"
          >
            close
          </button>
        </div>

        <div className="flex-1 overflow-y-auto overscroll-contain px-4 py-4 sm:px-5">
          <div className="text-[13px] leading-6 text-body">{summary}</div>

          <p className="label mt-5">{stepsLabel}</p>
          <div className="mt-2 max-h-[40vh] overflow-y-auto overscroll-contain rounded-lg border border-line bg-black/30 px-4 py-3 font-mono text-[12px] leading-6">
            {loading ? (
              <p className="text-muted">{loadingHint}</p>
            ) : stepBlock !== undefined ? (
              <pre className="whitespace-pre-wrap break-words text-body">{stepBlock}</pre>
            ) : stepLines && stepLines.length > 0 ? (
              <ol className="space-y-1.5">
                {stepLines.map((step, i) => (
                  <li key={i} className="flex gap-3 break-words text-body">
                    <span className="shrink-0 text-faint">{String(i + 1).padStart(2, "0")}</span>
                    <span>{step}</span>
                  </li>
                ))}
              </ol>
            ) : (
              <p className="text-muted">n/a</p>
            )}
          </div>
        </div>

        <div className="flex flex-wrap items-center justify-end gap-2 border-t border-line px-4 py-3 sm:px-5">
          <button
            onClick={onCancel}
            className="rounded-md border border-line px-4 py-2 text-[12px] text-muted transition-colors hover:border-line-strong hover:text-cream"
          >
            {cancelLabel}
          </button>
          <button
            onClick={onConfirm}
            disabled={loading}
            className={
              destructive
                ? "rounded-md border border-red-400/50 bg-red-500/10 px-4 py-2 text-[12px] font-medium text-red-300 transition-colors hover:bg-red-500/20 disabled:cursor-not-allowed disabled:opacity-40"
                : "rounded-md border border-lime/50 bg-lime/[0.08] px-4 py-2 text-[12px] font-medium text-lime transition-colors hover:bg-lime/[0.16] disabled:cursor-not-allowed disabled:opacity-40"
            }
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
