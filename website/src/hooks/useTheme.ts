import { useCallback, useState } from "react";

export type Theme = "dark" | "light";

const STORAGE_KEY = "aura-theme";

// The inline script in index.html applies the saved theme before first
// paint; this hook reads that state and keeps the DOM, the browser's
// color scheme, and localStorage in sync when the user toggles it.
function currentTheme(): Theme {
  return document.documentElement.classList.contains("dark") ? "dark" : "light";
}

function applyTheme(theme: Theme) {
  const root = document.documentElement;
  root.classList.toggle("dark", theme === "dark");
  root.style.colorScheme = theme;
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute("content", theme === "dark" ? "#06080f" : "#f7f8fc");
  try {
    localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    // Storage can be unavailable (private mode); the toggle still works
    // for this page view.
  }
}

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(currentTheme);
  const toggle = useCallback(() => {
    setTheme((prev) => {
      const next: Theme = prev === "dark" ? "light" : "dark";
      applyTheme(next);
      return next;
    });
  }, []);
  return { theme, toggle };
}
