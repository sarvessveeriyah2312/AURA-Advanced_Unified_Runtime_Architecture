import { motion, useReducedMotion } from "framer-motion";
import { ArrowDown, ArrowRight, Pause, Play } from "lucide-react";
import { useEffect, useState } from "react";
import { cn } from "../../lib/cn";
import { Reveal } from "../ui/Reveal";
import { Section, SectionHeading } from "../ui/SectionHeading";

interface Stage {
  name: string;
  group: "input" | "compiler" | "bytecode" | "runtime" | "output";
  detail: string;
}

const STAGES: Stage[] = [
  { name: "Source code", group: "input", detail: "A single .aura file, read as UTF-8 text. Line endings may be LF, CRLF, or CR." },
  { name: "Lexer", group: "compiler", detail: "Splits the text into tokens: keywords, names, literals, operators, and newlines, each with its exact source position." },
  { name: "Parser / AST", group: "compiler", detail: "Checks the grammar, decides which newlines end a statement, and builds an abstract syntax tree. It recovers so that several syntax errors are reported at once." },
  { name: "Type checker", group: "compiler", detail: "Resolves every name and checks every type, call, return path, and assignment. Any error stops the pipeline here." },
  { name: "Bytecode compiler", group: "compiler", detail: "Translates the checked program into typed instructions; for example, Int addition and Float addition are different opcodes." },
  { name: "AURA Bytecode", group: "bytecode", detail: "A compact list of typed instructions, validated before execution. It exists only in memory; AURA does not write bytecode files." },
  { name: "AURA Virtual Machine", group: "runtime", detail: "A stack-based interpreter. It reports overflow, division by zero, and excessive call depth as runtime errors, with stack traces." },
  { name: "Program output", group: "output", detail: "Output from print goes to stdout as the program runs; diagnostics go to stderr, with a documented exit code." },
];

const GROUP_LABEL: Record<Stage["group"], string> = {
  input: "Input",
  compiler: "AURA Compiler",
  bytecode: "Intermediate form",
  runtime: "Runtime",
  output: "Result",
};

export function Architecture() {
  const reduce = useReducedMotion();
  const [active, setActive] = useState(0);
  const [playing, setPlaying] = useState(!reduce);

  // Step through the pipeline automatically, unless the visitor prefers
  // reduced motion or has paused it. Choosing a stage also pauses it.
  useEffect(() => {
    if (!playing || reduce) return;
    const id = window.setInterval(() => setActive((i) => (i + 1) % STAGES.length), 2200);
    return () => window.clearInterval(id);
  }, [playing, reduce]);

  const stage = STAGES[active] ?? STAGES[0]!;

  return (
    <Section id="architecture" labelledBy="architecture-title" className="border-y border-line bg-surface-2/40">
      <SectionHeading
        id="architecture-title"
        eyebrow="Compiler architecture"
        title="From source to output, in eight steps."
        description="Each stage is a separate Go package with one job. The compiler finishes completely before the virtual machine starts."
      />

      <Reveal className="mt-12">
        <ol className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4 lg:gap-3">
          {STAGES.map((s, i) => {
            const isActive = i === active;
            return (
              <li key={s.name} className="relative">
                <button
                  type="button"
                  onClick={() => {
                    setActive(i);
                    setPlaying(false);
                  }}
                  aria-pressed={isActive}
                  aria-describedby={isActive ? "stage-detail" : undefined}
                  className={cn(
                    "relative flex w-full items-start gap-3 overflow-hidden rounded-xl border p-4 text-left transition-colors duration-300 lg:items-center",
                    isActive
                      ? "border-accent/60 bg-surface"
                      : "border-line bg-surface/60 hover:border-line-strong hover:bg-surface",
                  )}
                >
                  {isActive && (
                    <motion.span
                      layoutId="stage-glow"
                      className="absolute inset-0 bg-accent-soft"
                      transition={{ type: "spring", stiffness: 380, damping: 34 }}
                      aria-hidden
                    />
                  )}
                  <span
                    className={cn(
                      "relative flex size-8 shrink-0 items-center justify-center rounded-lg font-mono text-xs font-medium",
                      isActive ? "bg-accent text-white dark:text-[#0a0716]" : "bg-surface-2 text-subtle",
                    )}
                  >
                    {i + 1}
                  </span>
                  <span className="relative min-w-0">
                    <span className="block text-[11px] font-medium tracking-wide text-subtle uppercase">
                      {GROUP_LABEL[s.group]}
                    </span>
                    <span className="block truncate text-sm font-semibold">{s.name}</span>
                    {isActive && (
                      <span className="mt-1.5 block text-[13px] leading-relaxed text-muted lg:hidden">{s.detail}</span>
                    )}
                  </span>
                </button>
                {i < STAGES.length - 1 && (
                  <span className="pointer-events-none absolute top-full left-1/2 z-10 -translate-x-1/2 text-line-strong sm:hidden" aria-hidden>
                    <ArrowDown className="size-3" />
                  </span>
                )}
                {i < STAGES.length - 1 && (i + 1) % 4 !== 0 && (
                  <span className="pointer-events-none absolute top-1/2 -right-3 z-10 hidden -translate-y-1/2 text-line-strong lg:block" aria-hidden>
                    <ArrowRight className="size-3" />
                  </span>
                )}
              </li>
            );
          })}
        </ol>

        <div className="mt-4 flex flex-col gap-4 lg:mt-6 lg:grid lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start">
          <div id="stage-detail" aria-live="polite" className="hidden rounded-xl border border-line bg-surface p-5 lg:block">
            <p className="text-xs font-medium tracking-wide text-accent uppercase">
              Step {active + 1} · {GROUP_LABEL[stage.group]}
            </p>
            <p className="mt-1 font-semibold">{stage.name}</p>
            <p className="mt-2 text-sm leading-relaxed text-muted">{stage.detail}</p>
          </div>
          {!reduce && (
            <button
              type="button"
              onClick={() => setPlaying((p) => !p)}
              className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-line bg-surface px-4 text-sm text-muted transition-colors hover:text-fg"
              aria-label={playing ? "Pause the pipeline walkthrough" : "Play the pipeline walkthrough"}
            >
              {playing ? <Pause className="size-4" aria-hidden /> : <Play className="size-4" aria-hidden />}
              {playing ? "Pause" : "Play"}
            </button>
          )}
        </div>

        <p className="mt-6 text-sm text-muted">
          AURA does <strong className="font-semibold text-fg">not</strong> compile to native machine code:{" "}
          <code className="font-mono text-[13px]">aura run</code> compiles to in-memory bytecode and executes it on
          the AURA Virtual Machine each time.
        </p>
      </Reveal>
    </Section>
  );
}
