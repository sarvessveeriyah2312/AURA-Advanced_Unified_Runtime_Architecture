import { AnimatePresence, motion } from "framer-motion";
import { Info } from "lucide-react";
import { useRef, useState, type KeyboardEvent } from "react";
import { EXAMPLES } from "../../data/examples";
import { VERSION } from "../../data/site";
import { cn } from "../../lib/cn";
import { TreadCode, CopyButton, TerminalText, WindowBar } from "../ui/CodeBlock";
import { Reveal } from "../ui/Reveal";
import { Section, SectionHeading } from "../ui/SectionHeading";

export function CodeShowcase() {
  const [index, setIndex] = useState(0);
  const tabs = useRef<Array<HTMLButtonElement | null>>([]);
  const example = EXAMPLES[index] ?? EXAMPLES[0]!;

  // WAI-ARIA tabs: arrow keys move between tabs, Home and End jump.
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const last = EXAMPLES.length - 1;
    const targets: Record<string, number> = {
      ArrowRight: index === last ? 0 : index + 1,
      ArrowLeft: index === 0 ? last : index - 1,
      Home: 0,
      End: last,
    };
    const next = targets[e.key];
    if (next === undefined) return;
    e.preventDefault();
    setIndex(next);
    tabs.current[next]?.focus();
  };

  return (
    <Section id="examples" labelledBy="examples-title">
      <SectionHeading
        id="examples-title"
        eyebrow="Tread in action"
        title="Read it once, and you know what it does."
        description="Real Tread programs, each shown with the exact output of the tread command."
      />

      <Reveal className="mt-10">
        <div
          role="tablist"
          aria-label="Tread examples"
          onKeyDown={onKeyDown}
          className="scrollbar-thin -mx-4 flex gap-1 overflow-x-auto px-4 pb-2 sm:mx-0 sm:flex-wrap sm:px-0"
        >
          {EXAMPLES.map((ex, i) => (
            <button
              key={ex.id}
              ref={(el) => {
                tabs.current[i] = el;
              }}
              id={`tab-${ex.id}`}
              role="tab"
              type="button"
              aria-selected={i === index}
              aria-controls={`panel-${ex.id}`}
              tabIndex={i === index ? 0 : -1}
              onClick={() => setIndex(i)}
              className={cn(
                "h-9 shrink-0 rounded-lg px-3.5 text-sm font-medium transition-colors",
                i === index
                  ? "bg-fg text-bg"
                  : "text-muted hover:bg-surface-2 hover:text-fg",
              )}
            >
              {ex.label}
            </button>
          ))}
        </div>

        <div
          id={`panel-${example.id}`}
          role="tabpanel"
          aria-labelledby={`tab-${example.id}`}
          className="mt-4 overflow-hidden rounded-2xl border border-code-line bg-code-bg"
        >
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key={example.id}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.15 }}
              className="grid grid-cols-1 lg:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)]"
            >
              <div className="min-w-0 border-code-line lg:border-r">
                <WindowBar title={example.file}>
                  <CopyButton text={example.code} label={`Copy ${example.file} source`} />
                </WindowBar>
                <TreadCode code={example.code} />
              </div>
              <div className="flex min-w-0 flex-col border-t border-code-line lg:border-t-0">
                <div className="flex h-11 items-center justify-between border-b border-code-line bg-code-bar px-4">
                  <span className="font-mono text-xs text-code-dim">Output</span>
                  <span className="font-mono text-xs text-code-dim">exit code {example.exitCode}</span>
                </div>
                <TerminalText command={`tread ${example.command} ${example.file}`} output={example.output} className="flex-1" />
                <p className="border-t border-code-line px-4 py-3 text-xs leading-relaxed text-code-dim">{example.summary}</p>
              </div>
            </motion.div>
          </AnimatePresence>
        </div>

        <p className="mt-4 flex items-start gap-2 text-xs text-subtle">
          <Info className="mt-px size-3.5 shrink-0" aria-hidden />
          Example output, recorded from Tread {VERSION}. Nothing is executed in your browser. Install Tread to run these
          programs yourself.
        </p>
      </Reveal>
    </Section>
  );
}
