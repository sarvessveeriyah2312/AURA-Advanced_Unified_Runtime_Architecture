import { motion } from "framer-motion";
import { ArrowRight, Download, Monitor, Scale, Tag } from "lucide-react";
import { HERO_EXAMPLE } from "../../data/examples";
import { FULL_NAME, RELEASE_URL, REPO_URL, TAGLINE, VERSION } from "../../data/site";
import { ButtonLink } from "../ui/Button";
import { AuraCode, CopyButton, TerminalText, WindowBar } from "../ui/CodeBlock";
import { GitHubIcon } from "../ui/GitHubIcon";
import { Container } from "../ui/SectionHeading";

const ease = [0.22, 1, 0.36, 1] as const;

export function Hero() {
  return (
    <section aria-labelledby="hero-title" className="relative overflow-hidden">
      {/* Restrained backdrop: a faint grid and one soft accent glow. */}
      <div className="bg-grid pointer-events-none absolute inset-0 opacity-60 dark:opacity-40" aria-hidden />
      <div
        className="pointer-events-none absolute -top-40 left-1/2 h-[480px] w-[760px] -translate-x-1/2 rounded-full bg-accent/15 blur-[120px] dark:bg-accent/20"
        aria-hidden
      />

      <Container className="relative grid grid-cols-1 items-center gap-14 pt-16 pb-20 sm:pt-24 lg:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)] lg:gap-12 lg:pt-28 lg:pb-28">
        <motion.div
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease }}
        >
          <a
            href={RELEASE_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="group inline-flex items-center gap-2 rounded-full border border-line bg-surface/70 py-1 pr-3 pl-1 text-xs text-muted transition-colors hover:border-line-strong hover:text-fg"
          >
            <span className="rounded-full bg-accent-soft px-2 py-0.5 font-medium text-accent">New</span>
            AURA {VERSION} adds runtime stack traces
            <ArrowRight className="size-3.5 transition-transform group-hover:translate-x-0.5" aria-hidden />
            <span className="sr-only">(release notes open in a new tab)</span>
          </a>

          <h1 id="hero-title" className="mt-7 text-5xl font-semibold tracking-tight sm:text-6xl lg:text-7xl">
            Meet{" "}
            <span className="bg-gradient-to-r from-accent via-accent-2 to-electric bg-clip-text text-transparent">
              AURA.
            </span>
          </h1>
          <p className="mt-4 text-lg font-medium text-fg/90 sm:text-xl">{FULL_NAME}</p>
          <p className="mt-2 text-lg text-muted italic">{TAGLINE}</p>
          <p className="mt-6 max-w-xl text-base leading-relaxed text-pretty text-muted sm:text-[17px]">
            AURA is an independently developed, statically typed programming language built around its own compiler,
            bytecode instruction set, and stack-based virtual machine.
          </p>

          <div className="mt-9 flex flex-col gap-3 sm:flex-row">
            <ButtonLink href="#download" size="lg">
              <Download className="size-[18px]" aria-hidden />
              Download AURA
            </ButtonLink>
            <ButtonLink href={REPO_URL} variant="secondary" size="lg">
              <GitHubIcon className="size-[18px]" />
              Explore on GitHub
              <span className="sr-only">(opens in a new tab)</span>
            </ButtonLink>
          </div>

          <ul className="mt-9 flex flex-wrap gap-x-6 gap-y-3 text-sm text-muted" aria-label="Release facts">
            <li className="flex items-center gap-2">
              <Tag className="size-4 text-accent" aria-hidden />
              Version {VERSION}
            </li>
            <li className="flex items-center gap-2">
              <Scale className="size-4 text-accent" aria-hidden />
              MIT License
            </li>
            <li className="flex items-center gap-2">
              <Monitor className="size-4 text-accent" aria-hidden />
              macOS · Linux · Windows
            </li>
          </ul>
        </motion.div>

        <motion.figure
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7, ease, delay: 0.12 }}
          className="relative min-w-0"
        >
          <div className="overflow-hidden rounded-2xl border border-code-line bg-code-bg shadow-[0_30px_80px_-40px_rgba(80,60,200,0.55)]">
            <WindowBar title={HERO_EXAMPLE.file}>
              <CopyButton text={HERO_EXAMPLE.code} label="Copy greet.aura source" />
            </WindowBar>
            <AuraCode code={HERO_EXAMPLE.code} />
            <div className="border-t border-code-line">
              <TerminalText command={`aura ${HERO_EXAMPLE.command} ${HERO_EXAMPLE.file}`} output={HERO_EXAMPLE.output} />
            </div>
          </div>
          <figcaption className="mt-3 text-center text-xs text-subtle">
            Real AURA {VERSION} source and output, recorded from <code className="font-mono">aura run</code>.
          </figcaption>
        </motion.figure>
      </Container>
    </section>
  );
}
