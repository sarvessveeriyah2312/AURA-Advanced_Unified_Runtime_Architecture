import { BookOpen, Cpu, ShieldCheck, Workflow } from "lucide-react";
import { Card, IconBadge } from "../ui/Card";
import { Reveal } from "../ui/Reveal";
import { Section, SectionHeading } from "../ui/SectionHeading";

const PILLARS = [
  {
    icon: ShieldCheck,
    title: "Statically typed",
    body: "Every value has a type known before the program runs. Type errors are reported by the compiler, with an exact line and column, and nothing executes until they are fixed.",
  },
  {
    icon: BookOpen,
    title: "Readable by design",
    body: "A small, consistent syntax: newlines end statements, braces mark blocks, and bindings are immutable unless declared with let mut.",
  },
  {
    icon: Workflow,
    title: "Its own compiler",
    body: "AURA does not translate to another language. A dedicated lexer, parser, type checker, and bytecode compiler turn source code into AURA's own typed instructions.",
  },
  {
    icon: Cpu,
    title: "A stack-based VM",
    body: "The AURA Virtual Machine executes that bytecode on an operand stack, checking integer overflow, division by zero, and call depth as it runs.",
  },
];

export function Introduction() {
  return (
    <Section id="overview" labelledBy="overview-title">
      <div className="grid grid-cols-1 gap-12 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)] lg:gap-16">
        <div>
          <SectionHeading
            id="overview-title"
            eyebrow="What is AURA?"
            title="A small language, built end to end."
            description="AURA is a general-purpose programming language designed to be predictable and easy to understand, from the syntax you write to the virtual machine that runs it."
          />
          <Reveal delay={0.05} className="mt-6 space-y-4 text-[15px] leading-relaxed text-muted">
            <p>
              Its guiding idea is a core that one developer can read and extend. Types, operators, and errors are
              specified rather than left to convention. Integer overflow is an error, never a silent wrap-around.
            </p>
            <p>
              AURA is young and deliberately small. Version 0.x focuses on a solid foundation (types, functions,
              control flow, and diagnostics) before adding larger features such as loops and collections.
            </p>
          </Reveal>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {PILLARS.map((p, i) => (
            <Reveal key={p.title} delay={0.05 * i}>
              <Card interactive className="h-full">
                <IconBadge>
                  <p.icon className="size-5" />
                </IconBadge>
                <h3 className="mt-5 text-base font-semibold">{p.title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-muted">{p.body}</p>
              </Card>
            </Reveal>
          ))}
        </div>
      </div>
    </Section>
  );
}
