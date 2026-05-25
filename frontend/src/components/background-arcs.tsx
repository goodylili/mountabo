// Faint concentric "topographic" arcs anchored to the top-right, echoing the
// orbit lines in the mountabo product shots. Purely decorative.
export function BackgroundArcs() {
  const cx = 1540;
  const cy = 280;
  const rings = Array.from({ length: 11 }, (_, i) => 120 + i * 150);

  return (
    <div
      aria-hidden
      className="pointer-events-none fixed inset-0 -z-10 overflow-hidden"
    >
      <svg
        className="absolute inset-0 h-full w-full"
        viewBox="0 0 1920 1080"
        preserveAspectRatio="xMidYMin slice"
        fill="none"
      >
        {rings.map((r, i) => (
          <circle
            key={r}
            cx={cx}
            cy={cy}
            r={r}
            className={i % 4 === 0 ? "stroke-lime/[0.07]" : "stroke-cream/[0.05]"}
            strokeWidth={1}
          />
        ))}
      </svg>
      {/* subtle vignette so content stays legible over the arcs */}
      <div
        className="absolute inset-0"
        style={{
          background:
            "radial-gradient(120% 90% at 50% 120%, transparent 40%, var(--bg) 100%)",
        }}
      />
    </div>
  );
}
