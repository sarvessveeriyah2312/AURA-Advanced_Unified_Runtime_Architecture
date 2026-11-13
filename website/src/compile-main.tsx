import { StrictMode } from "react";
import { createRoot, hydrateRoot } from "react-dom/client";
import CompileApp from "./CompileApp";
import "./styles/index.css";

const root = document.getElementById("root");
if (!root) throw new Error("missing #root element");

const app = (
  <StrictMode>
    <CompileApp />
  </StrictMode>
);

// Production HTML is prerendered (scripts/prerender.mjs): hydrate it.
if (root.hasChildNodes()) hydrateRoot(root, app);
else createRoot(root).render(app);
