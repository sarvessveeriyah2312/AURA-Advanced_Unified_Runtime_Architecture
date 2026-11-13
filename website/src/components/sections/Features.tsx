import { Binary, Boxes, Braces, Cpu, Layers, ListTree, Shapes, ShieldCheck, SquareTerminal, TriangleAlert } from "lucide-react";
import { VERSION } from "../../data/site";
import { Card, IconBadge } from "../ui/Card";
import { Reveal } from "../ui/Reveal";
import { Section, SectionHeading } from "../ui/SectionHeading";

// Every item below is implemented and tested in the current release.
const FEATURES = [
  {
    icon: ShieldCheck,
    title: "Static type system",
    points: [
      "Int, Float, Bool, String, Unit, arrays, maps, enums, and structs",
      "Optional type annotations",
      "Local type inference from initializers",
      "Compile-time checking, with no implicit conversions",
    ],
  },
  {
    icon: Layers,
    title: "Custom compiler",
    points: [
      "Dedicated lexer",
      "Parser that builds an AST",
      "Type checker with name resolution",
      "Bytecode compiler that emits typed instructions",
    ],
  },
  {
    icon: Cpu,
    title: "Tread Virtual Machine",
    points: [
      "Stack-based execution",
      "70 bytecode instructions",
      "Bytecode validated before it runs",
      "Never panics on a user program",
    ],
  },
  {
    icon: ListTree,
    title: "Structured diagnostics",
    points: [
      "69 stable codes, such as TR-T001",
      "Exact line and column positions",
      "Underlined source excerpts and help text",
      "Several errors reported in one run",
    ],
  },
  {
    icon: Braces,
    title: "Functions",
    points: [
      "Function declarations in any order",
      "Typed parameters",
      "Explicit return types, checked on every path",
      "Recursion",
    ],
  },
  {
    icon: SquareTerminal,
    title: "Developer experience",
    points: [
      "tread check finds errors without running",
      "tread run checks, compiles, and executes",
      "Documented exit codes for scripts and CI",
      "Program output on stdout, errors on stderr",
    ],
  },
  {
    icon: Binary,
    title: "Checked arithmetic",
    points: [
      "Int overflow is a runtime error",
      "Division by zero is a runtime error",
      "IEEE-754 Float semantics",
      "Shortest round-trip Float formatting",
    ],
  },
  {
    icon: Boxes,
    title: "Collections, enums, and generics",
    badge: "Since 0.5.0",
    points: [
      "Array<T> and Map<K, V> with value semantics",
      "Enums whose variants carry values",
      "match, checked to cover every case",
      "Generic functions, with inferred types",
    ],
  },
  {
    icon: Shapes,
    title: "Structs, methods, and modules",
    badge: "Since 0.6.0",
    points: [
      "Structs with value semantics",
      "Methods in impl blocks, with mut self",
      "Programs split across files with import",
      "pub controls what other files can use",
    ],
  },
  {
    icon: TriangleAlert,
    title: "Stack traces",
    badge: "Since 0.2.0",
    points: [
      "Every active call, innermost first",
      "Function, location, and source line",
      "Recursion merged into a single line",
      "At most 20 lines, however deep",
    ],
  },
];

export function Features() {
  return (
    <Section id="features" labelledBy="features-title" className="border-y border-line bg-surface-2/40">
      <SectionHeading
        id="features-title"
        eyebrow="Features"
        title="What Tread does today."
        description={`Everything on this page is implemented and tested in Tread ${VERSION}. Planned work is listed separately in the roadmap.`}
      />
      <div className="mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {FEATURES.map((f, i) => (
          <Reveal key={f.title} delay={0.04 * (i % 4)}>
            <Card interactive className="h-full">
              <div className="flex items-start justify-between gap-3">
                <IconBadge>
                  <f.icon className="size-5" />
                </IconBadge>
                {f.badge && (
                  <span className="rounded-full bg-accent-soft px-2.5 py-1 text-[11px] font-medium text-accent">
                    {f.badge}
                  </span>
                )}
              </div>
              <h3 className="mt-5 font-semibold">{f.title}</h3>
              <ul className="mt-3 space-y-2 text-sm text-muted">
                {f.points.map((p) => (
                  <li key={p} className="flex gap-2.5">
                    <span className="mt-2 size-1 shrink-0 rounded-full bg-accent/70" aria-hidden />
                    <span>{p}</span>
                  </li>
                ))}
              </ul>
            </Card>
          </Reveal>
        ))}
      </div>
    </Section>
  );
}
