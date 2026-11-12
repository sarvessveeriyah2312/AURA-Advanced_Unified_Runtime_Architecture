import { useSyncExternalStore } from "react";

// usePrefersReducedMotion reports the visitor's reduced-motion setting.
// It reports false while prerendering and hydrating, then the real value,
// so the prerendered HTML and the first client render always match.
const QUERY = "(prefers-reduced-motion: reduce)";

function subscribe(onChange: () => void) {
  const media = window.matchMedia(QUERY);
  media.addEventListener("change", onChange);
  return () => media.removeEventListener("change", onChange);
}

export function usePrefersReducedMotion(): boolean {
  return useSyncExternalStore(
    subscribe,
    () => window.matchMedia(QUERY).matches,
    () => false,
  );
}
