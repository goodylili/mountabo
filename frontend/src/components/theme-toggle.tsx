"use client";

import { useSyncExternalStore } from "react";
import { Moon, Sun } from "@/components/icons";

// The <html data-theme> attribute is the source of truth (set pre-paint by the
// inline script in layout). We subscribe to it rather than mirroring it in
// state, so there's no setState-in-effect and no hydration mismatch.
function subscribe(onChange: () => void) {
  const observer = new MutationObserver(onChange);
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["data-theme"],
  });
  return () => observer.disconnect();
}

function getTheme(): "dark" | "light" {
  return document.documentElement.getAttribute("data-theme") === "light" ? "light" : "dark";
}

export function ThemeToggle() {
  const theme = useSyncExternalStore(subscribe, getTheme, () => "dark");

  const toggle = () => {
    const next = theme === "dark" ? "light" : "dark";
    document.documentElement.setAttribute("data-theme", next);
    try {
      localStorage.setItem("mountabo-theme", next);
    } catch {
      // ignore storage errors (private mode etc.)
    }
  };

  return (
    <button
      onClick={toggle}
      aria-label={`switch to ${theme === "dark" ? "light" : "dark"} mode`}
      className="transition-colors hover:text-cream"
    >
      {theme === "dark" ? <Sun /> : <Moon />}
    </button>
  );
}
