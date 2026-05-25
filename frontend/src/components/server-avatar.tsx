// Deterministic generative avatar (a "marble" of blurred color fields), seeded
// from the name. Generated locally as SVG, so it works offline and gives every
// server/app a distinctive profile picture instead of a flat letter tile.

const PALETTE = ["#b6f04a", "#4f9df5", "#2dd4bf", "#f5a524", "#fb7185", "#38bdf8"];
const SIZE = 80;

function hashCode(name: string): number {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = (hash << 5) - hash + name.charCodeAt(i);
    hash |= 0;
  }
  return Math.abs(hash);
}

function digit(n: number, ntn: number): number {
  return Math.floor((n / Math.pow(10, ntn)) % 10);
}

function unit(n: number, range: number, index?: number): number {
  const value = n % range;
  if (index && digit(n, index) % 2 === 0) return -value;
  return value;
}

function field(seed: number, i: number) {
  const n = seed * (i + 1);
  return {
    color: PALETTE[(seed + i) % PALETTE.length],
    tx: unit(n, SIZE / 8, 1),
    ty: unit(n, SIZE / 8, 2),
    rotate: unit(n, 360, 1),
    scale: 1.2 + unit(n, SIZE / 20) / 12,
  };
}

export function ServerAvatar({
  seed,
  size = "md",
}: {
  seed: string;
  size?: "md" | "lg";
}) {
  const h = hashCode(seed);
  const f = [field(h, 0), field(h, 1), field(h, 2)];
  const id = `av-${h}`;
  const dim = size === "lg" ? "h-9 w-9" : "h-8 w-8";

  return (
    <svg
      viewBox={`0 0 ${SIZE} ${SIZE}`}
      className={`${dim} shrink-0 rounded-md`}
      role="img"
      aria-label={`${seed} avatar`}
    >
      <mask id={`${id}-m`} maskUnits="userSpaceOnUse" x="0" y="0" width={SIZE} height={SIZE}>
        <rect width={SIZE} height={SIZE} rx={18} fill="#fff" />
      </mask>
      <g mask={`url(#${id}-m)`}>
        <rect width={SIZE} height={SIZE} fill={f[0].color} />
        <path
          filter={`url(#${id}-f)`}
          d="M0 0h80v80H0z"
          fill={f[1].color}
          transform={`translate(${f[1].tx} ${f[1].ty}) rotate(${f[1].rotate} 40 40) scale(${f[1].scale})`}
        />
        <circle
          filter={`url(#${id}-f)`}
          cx="40"
          cy="40"
          r="34"
          fill={f[2].color}
          style={{ mixBlendMode: "overlay" }}
          transform={`translate(${f[2].tx} ${f[2].ty}) rotate(${f[2].rotate} 40 40)`}
        />
      </g>
      <defs>
        <filter id={`${id}-f`} x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="9" />
        </filter>
      </defs>
    </svg>
  );
}
