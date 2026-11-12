import { useCallback, useEffect, useRef, useState } from "react";

// useCopy copies text to the clipboard and reports success for a short
// time. It falls back to a hidden textarea where the async Clipboard API
// is unavailable (for example, a page served over plain HTTP).
export function useCopy(resetMs = 1800) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(timer.current), []);

  const copy = useCallback(
    async (text: string) => {
      let ok = false;
      try {
        if (navigator.clipboard && window.isSecureContext) {
          await navigator.clipboard.writeText(text);
          ok = true;
        }
      } catch {
        ok = false;
      }
      if (!ok) {
        const area = document.createElement("textarea");
        area.value = text;
        area.setAttribute("readonly", "");
        area.style.position = "fixed";
        area.style.opacity = "0";
        document.body.appendChild(area);
        area.select();
        try {
          ok = document.execCommand("copy");
        } catch {
          ok = false;
        }
        document.body.removeChild(area);
      }
      if (ok) {
        setCopied(true);
        window.clearTimeout(timer.current);
        timer.current = window.setTimeout(() => setCopied(false), resetMs);
      }
      return ok;
    },
    [resetMs],
  );

  return { copied, copy };
}
