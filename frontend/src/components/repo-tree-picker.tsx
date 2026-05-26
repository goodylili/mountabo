"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { fetchRepoTree, type TreeEntry } from "@/lib/tree";

const inputCls =
  "w-full rounded-lg border border-line bg-surface-2 px-3 py-2.5 text-[13px] text-cream placeholder:text-muted focus:border-line-strong focus:outline-none";

// RepoTreePicker is an editable combobox over a repo's real tree. mode "dir"
// lists directories (plus "./" for the repo root); mode "file" lists files. The
// field stays free-text, picking from the dropdown only fills it in, so a path
// is never forced or auto-corrected. The tree loads once per repo+ref and is
// filtered client-side by what the user types.
export function RepoTreePicker({
  owner,
  repo,
  gitRef,
  mode,
  value,
  onChange,
  placeholder,
}: {
  owner: string;
  repo: string;
  gitRef: string;
  mode: "dir" | "file";
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
}) {
  const [open, setOpen] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);

  // Key the loaded tree to its repo+ref. While the stored result is for a stale
  // key, `entries` reads back as null ("loading"), so a repo/branch change shows
  // the loading state without a synchronous setState in the effect body.
  const treeKey = `${owner}/${repo}@${gitRef}`;
  const [result, setResult] = useState<{ key: string; entries: TreeEntry[] } | null>(null);
  const entries = result && result.key === treeKey ? result.entries : null;

  useEffect(() => {
    const ctrl = new AbortController();
    fetchRepoTree(owner, repo, gitRef, ctrl.signal)
      .then((t) => setResult({ key: treeKey, entries: t }))
      .catch((e) => {
        if ((e as { name?: string })?.name !== "AbortError") setResult({ key: treeKey, entries: [] });
      });
    return () => ctrl.abort();
  }, [owner, repo, gitRef, treeKey]);

  // Close the dropdown when clicking outside the combobox.
  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  const options = useMemo(() => {
    if (!entries) return [];
    const wantDir = mode === "dir";
    let paths = entries.filter((e) => e.dir === wantDir).map((e) => e.path).sort();
    if (wantDir) paths = [".", ...paths]; // the repo root
    const q = value.trim().toLowerCase().replace(/^\.\/?/, "").replace(/\/+$/, "");
    const matches = q ? paths.filter((p) => p.toLowerCase().includes(q)) : paths;
    return matches.slice(0, 200); // keep the list bounded on huge repos
  }, [entries, mode, value]);

  const noun = mode === "dir" ? "folders" : "files";

  return (
    <div ref={boxRef} className="relative">
      <input
        value={value}
        onChange={(e) => {
          onChange(e.target.value);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        placeholder={placeholder}
        spellCheck={false}
        className={`${inputCls} font-mono`}
      />
      {open && (
        <div className="absolute z-20 mt-1 max-h-64 w-full overflow-auto rounded-lg border border-line bg-surface-2 py-1 shadow-lg">
          {entries === null ? (
            <p className="px-3 py-2 text-[12px] text-muted">loading {noun} from the repo...</p>
          ) : options.length === 0 ? (
            <p className="px-3 py-2 text-[12px] text-muted">no {noun} match</p>
          ) : (
            options.map((p) => (
              <button
                key={p}
                type="button"
                onClick={() => {
                  onChange(mode === "dir" && p === "." ? "./" : p);
                  setOpen(false);
                }}
                style={{ paddingLeft: `${12 + pathDepth(p) * 14}px` }}
                className="flex w-full items-center gap-2 py-1.5 pr-3 text-left font-mono text-[12px] text-body transition-colors hover:bg-surface hover:text-cream"
              >
                <span className="shrink-0 text-faint">{mode === "dir" ? "\u{1F4C1}" : "\u{1F4C4}"}</span>
                <span className="truncate">{p === "." ? "./ (repo root)" : leaf(p)}</span>
              </button>
            ))
          )}
        </div>
      )}
    </div>
  );
}

// pathDepth indents a path by how deep it sits, so the dropdown reads as a tree.
function pathDepth(p: string): number {
  if (p === "." || p === "") return 0;
  return p.split("/").length - 1;
}

// leaf shows just the final path segment (the folder/file name) since depth is
// conveyed by indentation.
function leaf(p: string): string {
  const parts = p.split("/");
  return parts[parts.length - 1] || p;
}
