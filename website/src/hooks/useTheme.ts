import { useCallback, useSyncExternalStore } from "react";

export type Theme = "dark" | "light";

const STORAGE_KEY = "tread-theme";

// The inline script in index.html applies the saved theme before first
// paint. The page is also prerendered to static HTML at build time, where
// there is no document: the hook therefore reads the theme from the
// <html> class through useSyncExternalStore, reporting "light" (the
// default) during prerendering and hydration, then the real value. This
// keeps server and client markup identical and never flashes.
function currentTheme(): Theme {
  return document.documentElement.classList.contains("dark") ? "dark" : "light";
}

function subscribe(onChange: () => void) {
  const observer = new MutationObserver(onChange);
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
  return () => observer.disconnect();
}

const getServerSnapshot = (): Theme => "light";

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
  const theme = useSyncExternalStore(subscribe, currentTheme, getServerSnapshot);
  const toggle = useCallback(() => applyTheme(currentTheme() === "dark" ? "light" : "dark"), []);
  return { theme, toggle };
}
