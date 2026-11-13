// Prerenders every page after `vite build`: it puts each rendered page and
// its JSON-LD into its HTML file, and writes dist/sitemap.xml.
// Run by `npm run build`, after the client and server bundles are built.
import { readFileSync, rmSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const dist = fileURLToPath(new URL("../dist/", import.meta.url));
const serverDir = fileURLToPath(new URL("../dist-server/", import.meta.url));
const server = await import(new URL("../dist-server/entry-server.js", import.meta.url).href);
const { SITE_URL, UPDATED } = server;

const PAGES = [
  { file: "index.html", path: "", render: server.render, jsonLd: server.structuredData },
  { file: "changelog/index.html", path: "changelog/", render: server.renderChangelog, jsonLd: server.changelogStructuredData },
  { file: "download/index.html", path: "download/", render: server.renderDownload, jsonLd: server.downloadStructuredData },
  { file: "compile/index.html", path: "compile/", render: server.renderCompile, jsonLd: server.compileStructuredData },
];

const ROOT = '<div id="root"></div>';
for (const page of PAGES) {
  const file = dist + page.file;
  let html = readFileSync(file, "utf8");
  if (!html.includes(ROOT)) throw new Error(`prerender: empty #root not found in dist/${page.file}`);
  const app = page.render();
  if (app.length < 1000) throw new Error(`prerender: dist/${page.file} rendered suspiciously small (${app.length} bytes)`);
  html = html.replace(ROOT, `<div id="root">${app}</div>`);
  // JSON-LD, with "<" escaped so text can never close the script element.
  const jsonLd = JSON.stringify(page.jsonLd()).replace(/</g, "\\u003c");
  html = html.replace("</head>", `    <script type="application/ld+json">${jsonLd}</script>\n  </head>`);
  writeFileSync(file, html);
  const words = app.replace(/<[^>]+>/g, " ").split(/\s+/).filter(Boolean).length;
  console.log(`prerender: dist/${page.file} (${words} words)`);
}

const urls = PAGES.map((p) => `  <url>\n    <loc>${SITE_URL}${p.path}</loc>\n    <lastmod>${UPDATED}</lastmod>\n  </url>`).join("\n");
writeFileSync(
  dist + "sitemap.xml",
  `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${urls}\n</urlset>\n`,
);
rmSync(serverDir, { recursive: true, force: true });
console.log(`prerender: wrote dist/sitemap.xml (${PAGES.length} URLs)`);
