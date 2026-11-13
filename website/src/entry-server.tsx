// Build-time entry (not a hot-reloaded module), so re-exporting data is fine.
/* eslint-disable react-refresh/only-export-components */
import { StrictMode } from "react";
import { renderToString } from "react-dom/server";
import App from "./App";
import ChangelogApp from "./ChangelogApp";
import CompileApp from "./CompileApp";
import DownloadApp from "./DownloadApp";

export { changelogStructuredData, compileStructuredData, downloadStructuredData, structuredData } from "./lib/structuredData";
export { SITE_URL, UPDATED } from "./data/site";

// Build-time render of each page to static HTML (scripts/prerender.mjs).
// Crawlers and AI agents that do not run JavaScript then see the full
// content; the browser hydrates the same markup.
export function render(): string {
  return renderToString(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}

export function renderChangelog(): string {
  return renderToString(
    <StrictMode>
      <ChangelogApp />
    </StrictMode>,
  );
}

export function renderDownload(): string {
  return renderToString(
    <StrictMode>
      <DownloadApp />
    </StrictMode>,
  );
}

export function renderCompile(): string {
  return renderToString(
    <StrictMode>
      <CompileApp />
    </StrictMode>,
  );
}
