// Build-time entry (not a hot-reloaded module), so re-exporting data is fine.
/* eslint-disable react-refresh/only-export-components */
import { StrictMode } from "react";
import { renderToString } from "react-dom/server";
import App from "./App";

export { structuredData } from "./lib/structuredData";
export { SITE_URL, UPDATED } from "./data/site";

// Build-time render of the whole page to static HTML (scripts/prerender.mjs).
// Crawlers and AI agents that do not run JavaScript then see the full
// content; the browser hydrates the same markup (src/main.tsx).
export function render(): string {
  return renderToString(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}
