import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import type { MouseEvent } from "react";
import { Download, Menu, Moon, Sun, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { NAV_LINKS, REPO_URL } from "../../data/site";
import { useActiveSection } from "../../hooks/useActiveSection";
import { useTheme } from "../../hooks/useTheme";
import { cn } from "../../lib/cn";
import { ButtonLink } from "../ui/Button";
import { AuraMark, GitHubIcon } from "../ui/GitHubIcon";
import { Container } from "../ui/SectionHeading";

// Track every section (not only those in the nav), so that no nav link is
// highlighted while a section without one (CLI, Download, ...) is in view.
const SECTION_IDS = ["overview", "features", "examples", "architecture", "cli", "download", "documentation", "roadmap", "community"];

function ThemeToggle() {
  const { theme, toggle } = useTheme();
  const next = theme === "dark" ? "light" : "dark";
  return (
    <button
      type="button"
      onClick={toggle}
      className="inline-flex size-9 items-center justify-center rounded-lg text-muted transition-colors hover:bg-surface-2 hover:text-fg"
      aria-label={`Switch to ${next} theme`}
      title={`Switch to ${next} theme`}
    >
      {theme === "dark" ? <Sun className="size-[18px]" aria-hidden /> : <Moon className="size-[18px]" aria-hidden />}
    </button>
  );
}

export function Navbar() {
  const [open, setOpen] = useState(false);
  const pendingTarget = useRef<string | null>(null);
  const [scrolled, setScrolled] = useState(false);
  const active = useActiveSection(SECTION_IDS);
  const menuButton = useRef<HTMLButtonElement>(null);
  const reduceMotion = useReducedMotion();

  // A link in the mobile menu closes the menu first, then scrolls once
  // the menu has collapsed (see onExitComplete).
  const goFromMenu = (e: MouseEvent<HTMLAnchorElement>, id: string) => {
    e.preventDefault();
    pendingTarget.current = id;
    setOpen(false);
  };
  const scrollToPending = () => {
    const id = pendingTarget.current;
    pendingTarget.current = null;
    if (!id) return;
    document.getElementById(id)?.scrollIntoView({ behavior: reduceMotion ? "auto" : "smooth" });
    history.replaceState(null, "", `#${id}`);
  };

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 8);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  // Close the mobile menu on Escape, and when the viewport grows to the
  // desktop layout.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
        menuButton.current?.focus();
      }
    };
    const desktop = window.matchMedia("(min-width: 768px)");
    const onResize = () => desktop.matches && setOpen(false);
    window.addEventListener("keydown", onKey);
    desktop.addEventListener("change", onResize);
    return () => {
      window.removeEventListener("keydown", onKey);
      desktop.removeEventListener("change", onResize);
    };
  }, [open]);

  return (
    <header
      className={cn(
        "sticky top-0 z-50 border-b transition-colors duration-300",
        scrolled || open ? "border-line bg-bg/85 backdrop-blur-md" : "border-transparent bg-transparent",
      )}
    >
      <Container className="flex h-16 items-center justify-between gap-4">
        <a href="#top" className="flex items-center gap-2.5 rounded-md" aria-label="AURA, back to top">
          <AuraMark className="size-8" />
          <span className="text-[15px] font-semibold tracking-[0.12em]">AURA</span>
        </a>

        <nav aria-label="Primary" className="hidden md:block">
          <ul className="flex items-center gap-1">
            {NAV_LINKS.map((link) => (
              <li key={link.id}>
                <a
                  href={`#${link.id}`}
                  aria-current={active === link.id ? "location" : undefined}
                  className={cn(
                    "rounded-md px-3 py-2 text-sm transition-colors",
                    active === link.id ? "text-fg" : "text-muted hover:text-fg",
                  )}
                >
                  {link.label}
                </a>
              </li>
            ))}
          </ul>
        </nav>

        <div className="flex items-center gap-1.5">
          <ThemeToggle />
          {/* Visibility lives on a wrapper: the buttons' own display class
              would otherwise compete with "hidden". */}
          <div className="hidden items-center gap-1.5 sm:flex">
            <ButtonLink href={REPO_URL} variant="ghost" size="sm" aria-label="AURA on GitHub (opens in a new tab)">
              <GitHubIcon className="size-4" />
              GitHub
            </ButtonLink>
            <ButtonLink href="#download" size="sm">
              <Download className="size-4" aria-hidden />
              Download AURA
            </ButtonLink>
          </div>
          <button
            ref={menuButton}
            type="button"
            className="inline-flex size-9 items-center justify-center rounded-lg text-muted hover:bg-surface-2 hover:text-fg md:hidden"
            aria-expanded={open}
            aria-controls="mobile-menu"
            aria-label={open ? "Close menu" : "Open menu"}
            onClick={() => setOpen((v) => !v)}
          >
            {open ? <X className="size-5" aria-hidden /> : <Menu className="size-5" aria-hidden />}
          </button>
        </div>
      </Container>

      <AnimatePresence onExitComplete={scrollToPending}>
        {open && (
          <motion.nav
            id="mobile-menu"
            aria-label="Mobile"
            className="overflow-hidden border-t border-line md:hidden"
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.22, ease: "easeOut" }}
          >
            <Container className="py-3">
              <ul className="flex flex-col">
                {NAV_LINKS.map((link) => (
                  <li key={link.id}>
                    <a
                      href={`#${link.id}`}
                      onClick={(e) => goFromMenu(e, link.id)}
                      className={cn(
                        "block rounded-lg px-3 py-3 text-[15px]",
                        active === link.id ? "bg-surface-2 text-fg" : "text-muted hover:bg-surface-2 hover:text-fg",
                      )}
                    >
                      {link.label}
                    </a>
                  </li>
                ))}
              </ul>
              <div className="mt-3 grid grid-cols-2 gap-2 border-t border-line pt-3">
                <ButtonLink href={REPO_URL} variant="secondary" onClick={() => setOpen(false)}>
                  <GitHubIcon className="size-4" />
                  GitHub
                </ButtonLink>
                <ButtonLink href="#download" onClick={(e) => goFromMenu(e, "download")}>
                  <Download className="size-4" aria-hidden />
                  Download
                </ButtonLink>
              </div>
            </Container>
          </motion.nav>
        )}
      </AnimatePresence>
    </header>
  );
}
