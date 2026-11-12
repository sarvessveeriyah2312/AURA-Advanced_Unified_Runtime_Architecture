// Prerenders the site after `vite build`: it puts the rendered page and its
// JSON-LD into dist/index.html, and writes dist/sitemap.xml.
// Run by `npm run build`, after the client and server bundles are built.
import { readFileSync, rmSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const dist = fileURLToPath(new URL("../dist/", import.meta.url));
const serverDir = fileURLToPath(new URL("../dist-server/", import.meta.url));
const { render, structuredData, SITE_URL, UPDATED } = await import(new URL("../dist-server/entry-server.js", import.meta.url).href);

const indexPath = dist + "index.html";
let html = readFileSync(indexPath, "utf8");

const ROOT = '<div id="root"></div>';
if (!html.includes(ROOT)) throw new Error("prerender: empty #root not found in dist/index.html");
const app = render();
if (app.length < 1000) throw new Error(`prerender: rendered page is suspiciously small (${app.length} bytes)`);
html = html.replace(ROOT, `<div id="root">${app}</div>`);

// JSON-LD, with "<" escaped so text can never close the script element.
const jsonLd = JSON.stringify(structuredData()).replace(/</g, "\\u003c");
html = html.replace("</head>", `    <script type="application/ld+json">${jsonLd}</script>\n  </head>`);
writeFileSync(indexPath, html);

writeFileSync(
  dist + "sitemap.xml",
  `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>${SITE_URL}</loc>
    <lastmod>${UPDATED}</lastmod>
  </url>
</urlset>
`,
);

rmSync(serverDir, { recursive: true, force: true });
const words = app.replace(/<[^>]+>/g, " ").split(/\s+/).filter(Boolean).length;
console.log(`prerender: dist/index.html now contains the rendered page (${words} words) and JSON-LD; wrote dist/sitemap.xml`);
