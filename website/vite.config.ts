import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// base "./" keeps the build portable: it works from any path, including
// a GitHub Pages project site, with no server configuration.
export default defineConfig({
  base: "./",
  plugins: [react(), tailwindcss()],
});
