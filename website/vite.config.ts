import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// Three prerendered pages: home, changelog and download (paths are
// relative to this folder). base "/" because the site is served from the
// root of its domain.
export default defineConfig({
  base: "/",
  plugins: [react(), tailwindcss()],
  build: {
    rollupOptions: {
      input: { main: "index.html", changelog: "changelog/index.html", download: "download/index.html" },
    },
  },
  // The changelog page reads ../CHANGELOG.md (the repository's changelog).
  server: { fs: { allow: [".."] } },
});
