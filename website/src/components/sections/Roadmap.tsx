import { CircleCheck, CircleDot, Circle } from "lucide-react";
import { readmeSection } from "../../data/site";
import { cn } from "../../lib/cn";
import { Card } from "../ui/Card";
import { Reveal } from "../ui/Reveal";
import { Section, SectionHeading } from "../ui/SectionHeading";

// Mirrors the roadmap in the repository README. No dates are promised.
type Status = "Released" | "Planned" | "Under consideration";

const MILESTONES: { title: string; status: Status; version?: string; items: string[] }[] = [
  {
    title: "Language core",
    status: "Released",
    version: "0.1.0",
    items: [
      "Static types with inference",
      "Functions and recursion",
      "if / else if / else",
      "Strings with interpolation",
      "Bytecode compiler and stack VM",
      "tread run and tread check",
    ],
  },
  {
    title: "Runtime stack traces",
    status: "Released",
    version: "0.2.0",
    items: ["Every active call in runtime errors", "Recursion merged into one line", "Bounded to 20 lines"],
  },
  {
    title: "Loops, SDK and Runtime",
    status: "Released",
    version: "0.4.0",
    items: [
      "while and for i in a..b",
      "break and continue",
      ".tbc bytecode files",
      "SDK and Runtime installers",
    ],
  },
  {
    title: "Collections, enums, generics",
    status: "Released",
    version: "0.5.0",
    items: ["Array<T> and Map<K, V>", "Enums and exhaustive match", "Generic functions and Option<T>"],
  },
  {
    title: "Structs, methods, imports",
    status: "Released",
    version: "0.6.0",
    items: ["Structs with value semantics", "Methods in impl blocks", "Multi-file programs with import and pub"],
  },
  {
    title: "Formatter, tests, editor support",
    status: "Released",
    version: "0.7.0",
    items: ["tread fmt", "test blocks and tread test", "tread lsp and a VS Code extension"],
  },
  {
    title: "Next up",
    status: "Planned",
    items: [
      "File and network I/O",
      "Structured error handling (Result)",
      "Package ecosystem",
    ],
  },
];

const CONSIDERING = ["Native compilation"];

const STYLE: Record<Status, { icon: typeof Circle; className: string }> = {
  Released: { icon: CircleCheck, className: "text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 border-emerald-500/25" },
  Planned: { icon: CircleDot, className: "text-accent bg-accent-soft border-accent/30" },
  "Under consideration": { icon: Circle, className: "text-muted bg-surface-2 border-line" },
};

function StatusBadge({ status }: { status: Status }) {
  const { icon: Icon, className } = STYLE[status];
  return (
    <span className={cn("inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium", className)}>
      <Icon className="size-3.5" aria-hidden />
      {status}
    </span>
  );
}

export function Roadmap() {
  return (
    <Section id="roadmap" labelledBy="roadmap-title" className="border-y border-line bg-surface-2/40">
      <SectionHeading
        id="roadmap-title"
        eyebrow="Roadmap"
        title="Where Tread is going."
        description={
          <>
            Released work is in the current version. Planned and possible features are not available yet, and there
            are no promised dates. The{" "}
            <a href={readmeSection("roadmap")} target="_blank" rel="noopener noreferrer" className="text-accent hover:underline">
              README roadmap
            </a>{" "}
            is the source of truth.
          </>
        }
      />

      <ol className="mt-12 grid grid-cols-1 gap-4 md:grid-cols-3">
        {MILESTONES.map((m, i) => (
          <li key={m.title}>
            <Reveal delay={0.05 * i} className="h-full">
              <Card className={cn("h-full", m.status === "Planned" && "border-dashed")}>
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <StatusBadge status={m.status} />
                  {m.version && <span className="font-mono text-xs text-subtle">v{m.version}</span>}
                </div>
                <h3 className="mt-4 font-semibold">{m.title}</h3>
                <ul className="mt-3 space-y-1.5 text-sm text-muted">
                  {m.items.map((it) => (
                    <li key={it} className="flex gap-2.5">
                      <span className="mt-2 size-1 shrink-0 rounded-full bg-line-strong" aria-hidden />
                      {it}
                    </li>
                  ))}
                </ul>
              </Card>
            </Reveal>
          </li>
        ))}
      </ol>

      <Reveal className="mt-4">
        <Card>
          <div className="flex flex-wrap items-center gap-3">
            <StatusBadge status="Under consideration" />
            <p className="text-sm text-muted">Ideas being evaluated. None of these are available in Tread today.</p>
          </div>
          <ul className="mt-4 flex flex-wrap gap-2">
            {CONSIDERING.map((c) => (
              <li key={c} className="rounded-lg border border-line bg-surface-2/60 px-3 py-1.5 text-sm text-muted">
                {c}
              </li>
            ))}
          </ul>
        </Card>
      </Reveal>
    </Section>
  );
}
