import { MotionConfig } from "framer-motion";
import { Footer } from "./components/layout/Footer";
import { Navbar } from "./components/layout/Navbar";
import { Home } from "./pages/Home";

// A single page: in-page anchors are enough, so there is no router.
export default function App() {
  return (
    <MotionConfig reducedMotion="user">
      <div id="top" className="min-h-screen overflow-x-clip">
        <a
          href="#main"
          className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-[60] focus:rounded-lg focus:bg-surface focus:px-4 focus:py-2 focus:text-sm focus:shadow-card"
        >
          Skip to content
        </a>
        <Navbar />
        <main id="main">
          <Home />
        </main>
        <Footer />
      </div>
    </MotionConfig>
  );
}
