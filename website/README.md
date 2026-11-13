# Tread website

The official landing page for **Tread**, a language for learning how
languages work. It is a static, single-page site built with React,
Vite, TypeScript, Tailwind CSS, Framer Motion, and Lucide icons. There is
no backend.

## Commands

``` bash
npm install
npm run dev              # local development server
npm run build            # type-check, then production build into dist/
npm run preview          # serve dist/ locally
npm run typecheck        # TypeScript only
npm run lint             # ESLint
npm run verify:examples  # run every Tread example against a real tread binary
```

`npm run verify:examples` uses `tread` from your `PATH`, or the binary
named by `Tread=/path/to/tread`. Each example's output and exit code must
match the text shown on the site exactly.

## Pages

-   `/` (`index.html`, `src/App.tsx`): the home page.
-   `/changelog/` (`changelog/index.html`, `src/ChangelogApp.tsx`): the
    changelog. It is generated at build time from the repository's
    `../CHANGELOG.md` by `src/data/changelog.ts`, which parses exactly
    the Markdown that file uses. Edit `CHANGELOG.md`, never the page.

-   `/download/` (`download/index.html`, `src/DownloadApp.tsx`): Tread
    SDK and Tread Runtime installers and archives for every platform.
    Its data is `src/data/downloads.ts`; file names follow
    `scripts/build-release.sh` and `packaging/` exactly. The OS tab is
    picked in the browser (`detectOs`); the prerendered page shows macOS.

Every page shares `SiteShell` (navigation, footer). Navigation links use
`/#section` hrefs, so they work from either page. The site is served from
the root of its domain (`base: "/"` in `vite.config.ts`).

The build reads `../CHANGELOG.md`, outside this folder. On Vercel, keep
"Include files outside the root directory in the Build Step" enabled
(the default).

## Search and AI visibility

Every page is **prerendered**. `npm run build` builds the client bundle,
then a server bundle (`src/entry-server.tsx`), and then
`scripts/prerender.mjs` does three things:

1.  renders the whole page into `dist/index.html`, so crawlers and AI
    agents that do not run JavaScript see the full content; the browser
    then hydrates the same markup (`src/main.tsx`);
2.  injects JSON-LD structured data (`src/lib/structuredData.ts`):
    `WebSite`, `ComputerLanguage`, `SoftwareSourceCode`, `Person`, and
    `FAQPage`, built from the same data as the visible text;
3.  writes `dist/sitemap.xml`, listing both pages, with `<lastmod>`
    taken from `UPDATED` in `src/data/site.ts`.

The changelog page gets its own title, description, canonical URL, and
`WebPage` and `BreadcrumbList` JSON-LD (`changelog/index.html`,
`changelogStructuredData`).

Static discovery files live in `public/`:

-   `robots.txt`: allows every crawler, and points to the sitemap;
-   `llms.txt`: a plain-text overview for AI assistants (llmstxt.org).
    It is not shown to visitors; each page links it from its `<head>`
    (`<link rel="alternate">`), which is how crawlers discover it;
-   `og-image.png`: the 1200×630 social sharing card.

**The domain appears in several places.** `https://treadlang.vercel.app/`
is `SITE_URL` in `src/data/site.ts`, and it also appears as a literal in
`index.html` (canonical, Open Graph, and Twitter tags), `public/robots.txt`,
and `public/llms.txt`. Change them all together.

**On release day**, set `VERSION` (and `RELEASE_DATE`) in
`src/data/site.ts`. The download page, the home page's archive names and
the "Download Tread" button all follow `VERSION`. Then update the facts
in `public/llms.txt`, rerun `npm run verify:examples` against the new
`tread` binary, and rebuild the playground (`scripts/build-playground.sh`
in the repository root).

Components must not read `window`, `document`, or `localStorage` while
rendering, because the page is rendered at build time. Read them in
effects, or with `useSyncExternalStore` and a server snapshot, as
`useTheme` and `usePrefersReducedMotion` do.

## Deploying

`npm run build` writes a self-contained static site to `dist/`.
`vercel.json` gives the content-hashed files in `/assets/` a one-year
immutable cache. Asset
paths are relative (`base: "./"`), so `dist/` works from any URL path,
including a GitHub Pages project site, with no server configuration.

## Updating for a new Tread release

All facts shown on the site live in `src/data/`:

-   **`site.ts`**
    -   `VERSION`, `RELEASED` (set to `true` once the GitHub release exists), and
        `RELEASE_DATE`;
    -   repository and release URLs, built from the version;
    -   the platform archive names.

    Check every new asset name against the GitHub release.
-   **`examples.ts`**
    -   the hero program and the example tabs;
    -   the real output of each, which must be re-recorded with the new
        `tread`. Then run `npm run verify:examples`.
    -   the `tread help` text.
-   **Feature and roadmap content** is in
    `src/components/sections/Features.tsx` and `Roadmap.tsx`. Keep it in
    line with the repository README.

## Structure

``` text
src/
  data/                 site facts and Tread examples (single source of truth)
  lib/highlight.ts      dependency-free Tread syntax highlighter
  hooks/                theme (localStorage), clipboard, active section
  components/
    layout/             Navbar (sticky, mobile menu), Footer
    sections/           Hero, Introduction, Features, CodeShowcase, Architecture,
                        CLIShowcase, Downloads, Documentation, Roadmap, Community
    ui/                 Button, Card, CodeBlock, SectionHeading, Reveal, icons
  pages/Home.tsx        section order
  styles/index.css      design tokens (dark and light), Tailwind setup
scripts/verify-examples.mjs
go.mod                  stub: keeps website/ out of the Tread Go module
```

## Design notes

-   **Theme.** Light by default, with an explicit toggle and no "system"
    option. The choice is stored in `localStorage` (`tread-theme`), and an
    inline script in `index.html` applies it before first paint.
-   **Motion.** Motion is subtle and respects `prefers-reduced-motion`:
    -   reveal animations are skipped;
    -   the architecture walkthrough does not auto-play;
    -   smooth scrolling is disabled.
-   **Honest examples.** Code samples are real Tread. Their output is
    labelled as recorded output; nothing executes in the browser.
-   **Accessibility.** The site includes:
    -   semantic sections with one `h1`;
    -   a skip link;
    -   keyboard-operable tabs (arrow keys, Home, End);
    -   `aria-current` on the active nav link;
    -   labelled icon buttons;
    -   `rel="noopener"` on external links.
