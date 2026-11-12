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

## Deploying

`npm run build` writes a self-contained static site to `dist/`. Asset
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
