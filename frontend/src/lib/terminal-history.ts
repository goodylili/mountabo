// Persistent history of typed commands and AI prompts for the terminal page.
// Shell-style: each input keeps its own ring of recent entries the operator can
// scroll back through with the up/down arrow keys. State lives in localStorage
// so a refresh, a different tab, or a server restart does not lose the trail.
//
// Commands are kept per server (different machines have different vocabularies)
// and AI prompts are global (the natural-language request is server-agnostic).

const PREFIX = "mountabo:terminal:";

// Caps per list, large enough to be useful, small enough to keep localStorage
// reads cheap. Older entries are evicted from the front as new ones come in.
export const COMMAND_HISTORY_LIMIT = 200;
export const AI_HISTORY_LIMIT = 100;

// Storage-key helpers. Keeping the server id in the command key means switching
// servers loads that server's own history rather than mixing them together.
export function commandHistoryKey(serverId: string): string {
  return `${PREFIX}commands:${serverId}`;
}
export const AI_HISTORY_KEY = `${PREFIX}ai-prompts`;

// readHistory loads a history list. A missing or unreadable entry is treated as
// empty, so storage being disabled (private mode, quota) never breaks the page.
export function readHistory(key: string): string[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((x): x is string => typeof x === "string");
  } catch {
    return [];
  }
}

// appendHistory pushes one entry to the named list, returning the new list.
// Blank entries are ignored; an exact-match repeat of the previous entry is
// collapsed (matches what bash does with HISTCONTROL=ignoredups). Older entries
// are evicted past the cap so the list never grows unbounded.
export function appendHistory(key: string, entry: string, limit: number): string[] {
  const trimmed = entry.trim();
  if (!trimmed) return readHistory(key);
  const current = readHistory(key);
  if (current[current.length - 1] === trimmed) return current;
  const next = [...current, trimmed].slice(-limit);
  if (typeof window !== "undefined") {
    try {
      window.localStorage.setItem(key, JSON.stringify(next));
    } catch {
      // storage disabled or over quota: history just is not persisted
    }
  }
  return next;
}
