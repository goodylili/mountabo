import type { SVGProps } from "react";

type IconProps = SVGProps<SVGSVGElement>;

const base = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.6,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
};

export function LogoMark({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={20} height={20} className={className} {...props}>
      <path
        d="M2 19 L9 6 L13 13 L16 9 L22 19 Z"
        fill="none"
        stroke="currentColor"
        strokeWidth={1.6}
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function GithubMark({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 16 16" width={16} height={16} className={className} {...props}>
      <path
        fill="currentColor"
        d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z"
      />
    </svg>
  );
}

export function ChevronDown({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={16} height={16} className={className} {...base} {...props}>
      <path d="m6 9 6 6 6-6" />
    </svg>
  );
}

export function ChevronRight({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={16} height={16} className={className} {...base} {...props}>
      <path d="m9 6 6 6-6 6" />
    </svg>
  );
}

export function Lock({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={13} height={13} className={className} {...base} {...props}>
      <rect x="5" y="11" width="14" height="9" rx="1.5" />
      <path d="M8 11V8a4 4 0 0 1 8 0v3" />
    </svg>
  );
}

export function Shield({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={14} height={14} className={className} {...base} {...props}>
      <path d="M12 3 5 6v5c0 4.2 2.9 7.6 7 9 4.1-1.4 7-4.8 7-9V6l-7-3Z" />
    </svg>
  );
}

export function Book({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={16} height={16} className={className} {...base} {...props}>
      <path d="M4 5.5A2.5 2.5 0 0 1 6.5 3H20v15H6.5A2.5 2.5 0 0 0 4 20.5z" />
      <path d="M4 20.5A2.5 2.5 0 0 1 6.5 18H20" />
    </svg>
  );
}

export function Logout({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={16} height={16} className={className} {...base} {...props}>
      <path d="M15 4h3a1 1 0 0 1 1 1v14a1 1 0 0 1-1 1h-3" />
      <path d="M10 8 6 12l4 4" />
      <path d="M6 12h11" />
    </svg>
  );
}

export function Search({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={16} height={16} className={className} {...base} {...props}>
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-3.2-3.2" />
    </svg>
  );
}

export function Refresh({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={14} height={14} className={className} {...base} {...props}>
      <path d="M4 9a8 8 0 0 1 13.6-3.6L20 8" />
      <path d="M20 4v4h-4" />
      <path d="M20 15a8 8 0 0 1-13.6 3.6L4 16" />
      <path d="M4 20v-4h4" />
    </svg>
  );
}

export function Gear({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={14} height={14} className={className} {...base} {...props}>
      <circle cx="12" cy="12" r="3" />
      <path d="M12 2v3M12 19v3M2 12h3M19 12h3M5 5l2 2M17 17l2 2M19 5l-2 2M7 17l-2 2" />
    </svg>
  );
}

export function Check({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={16} height={16} className={className} {...base} {...props}>
      <path d="m5 12 5 5L20 6" />
    </svg>
  );
}

export function ArrowRight({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={16} height={16} className={className} {...base} {...props}>
      <path d="M4 12h16M14 6l6 6-6 6" />
    </svg>
  );
}

export function Plus({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={14} height={14} className={className} {...base} {...props}>
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

export function Sun({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={16} height={16} className={className} {...base} {...props}>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4 12H2M22 12h-2M5 5l1.5 1.5M17.5 17.5 19 19M19 5l-1.5 1.5M6.5 17.5 5 19" />
    </svg>
  );
}

export function Moon({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={16} height={16} className={className} {...base} {...props}>
      <path d="M20 14.5A8 8 0 0 1 9.5 4a7 7 0 1 0 10.5 10.5Z" />
    </svg>
  );
}

export function Branch({ className, ...props }: IconProps) {
  return (
    <svg viewBox="0 0 24 24" width={13} height={13} className={className} {...base} {...props}>
      <circle cx="6" cy="6" r="2.4" />
      <circle cx="6" cy="18" r="2.4" />
      <circle cx="18" cy="8" r="2.4" />
      <path d="M6 8.4v7.2M8.4 6.6c5 0 7.6.8 7.6 4.4 0 2-1.4 3-4 3.4" />
    </svg>
  );
}
