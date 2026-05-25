type Tone = "blue" | "gray" | "lime" | "lime-solid" | "red";

const toneClass: Record<Tone, string> = {
  blue: "border-blue/30 text-blue",
  gray: "border-line-strong text-muted",
  lime: "border-lime/40 text-lime",
  "lime-solid": "border-transparent bg-lime/15 text-lime",
  red: "border-red-500/40 text-red-400",
};

const dotClass: Record<Tone, string> = {
  blue: "bg-blue",
  gray: "bg-muted",
  lime: "bg-lime",
  "lime-solid": "bg-lime",
  red: "bg-red-400",
};

export function Badge({
  children,
  tone = "gray",
  dot = false,
}: {
  children: React.ReactNode;
  tone?: Tone;
  dot?: boolean;
}) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-[5px] border px-2 py-0.5 text-[10px] font-medium ${toneClass[tone]}`}
    >
      {dot && <span className={`h-1.5 w-1.5 rounded-full ${dotClass[tone]}`} />}
      {children}
    </span>
  );
}
