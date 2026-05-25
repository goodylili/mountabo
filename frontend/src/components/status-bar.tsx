// Bottom terminal status line, à la a code editor. Presentational only.
export type StatusSegment = { label: string; tone?: "muted" | "cream" | "lime" | "blue" };

const toneClass = {
  muted: "text-muted",
  cream: "text-cream",
  lime: "text-lime",
  blue: "text-blue",
} as const;

export function StatusBar({
  pill,
  left,
  right,
}: {
  pill?: string;
  left: StatusSegment[];
  right: StatusSegment[];
}) {
  return (
    <footer className="flex h-9 shrink-0 items-center gap-4 border-t border-line bg-surface px-4 text-[11px]">
      {pill && (
        <span className="flex items-center gap-2 rounded-[5px] bg-lime-fill px-2.5 py-1 font-medium text-black">
          <span className="h-1.5 w-1.5 rounded-full bg-black/70" />
          {pill}
        </span>
      )}
      {left.map((s, i) => (
        <span key={`l-${i}`} className="flex items-center gap-2">
          {i > 0 && <span className="text-faint">·</span>}
          <span className={toneClass[s.tone ?? "muted"]}>{s.label}</span>
        </span>
      ))}
      <span className="ml-auto flex items-center gap-4">
        {right.map((s, i) => (
          <span key={`r-${i}`} className={toneClass[s.tone ?? "muted"]}>
            {s.label}
          </span>
        ))}
      </span>
    </footer>
  );
}
