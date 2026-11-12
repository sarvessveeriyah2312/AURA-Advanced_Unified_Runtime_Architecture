import { MotionConfig } from "framer-motion";
import type { ReactNode } from "react";
import type { PageId } from "../../data/site";
import { Footer } from "./Footer";
import { Navbar } from "./Navbar";

// The frame shared by every page: skip link, navigation, main, footer.
export function SiteShell({ page, children }: { page: PageId; children: ReactNode }) {
  return (
    <MotionConfig reducedMotion="user">
      <div id="top" className="min-h-screen overflow-x-clip">
        <a
          href="#main"
          className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-[60] focus:rounded-lg focus:bg-surface focus:px-4 focus:py-2 focus:text-sm focus:shadow-card"
        >
          Skip to content
        </a>
        <Navbar page={page} />
        <main id="main">{children}</main>
        <Footer />
      </div>
    </MotionConfig>
  );
}
