import { useState } from "react";
import { CLI_HELP } from "../../data/examples";
import { VERSION } from "../../data/site";
import { cn } from "../../lib/cn";
import { TerminalText, WindowBar } from "../ui/CodeBlock";
import { Reveal } from "../ui/Reveal";
import { Section, SectionHeading } from "../ui/SectionHeading";

// Commands and their real output (tread 0.3.0).
const COMMANDS = [
  {
    command: "tread version",
    purpose: "Print the installed version. Also available as tread --version.",
    output: `tread ${VERSION}`,
  },
  {
    command: "tread check hello.tr",
    purpose: "Check a program for errors without running it. It prints nothing when the program is correct.",
    output: "",
    after: "$ echo $?\n0",
  },
  {
    command: "tread run hello.tr",
    purpose: "Check, compile, and run a program. It runs only if there are no compile-time errors.",
    output: "Hello, Tread!",
  },
  {
    command: "tread help",
    purpose: "Print usage. Also available as tread -h and tread --help.",
    output: CLI_HELP,
  },
];

const EXIT_CODES = [
  { code: "0", meaning: "Success" },
  { code: "1", meaning: "The program has compile-time or runtime errors" },
  { code: "2", meaning: "Usage error, a file not ending in .tr, or an unreadable file" },
  { code: "3", meaning: "Internal failure" },
];

export function CLIShowcase() {
  const [index, setIndex] = useState(0);
  const selected = COMMANDS[index] ?? COMMANDS[0]!;
  const output = [selected.output, selected.after].filter((s) => s !== undefined && s !== "").join("\n");

  return (
    <Section id="cli" labelledBy="cli-title">
      <SectionHeading
        id="cli-title"
        eyebrow="CLI experience"
        title="One command, four verbs."
        description="The tread command is the whole toolchain. Program output goes to stdout and diagnostics to stderr, so it fits naturally into scripts and CI."
      />

      <Reveal className="mt-12 grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <ul className="flex flex-col gap-2" aria-label="tread commands">
          {COMMANDS.map((c, i) => (
            <li key={c.command}>
              <button
                type="button"
                onClick={() => setIndex(i)}
                aria-pressed={i === index}
                className={cn(
                  "w-full rounded-xl border p-4 text-left transition-colors",
                  i === index ? "border-accent/60 bg-accent-soft" : "border-line bg-surface hover:border-line-strong",
                )}
              >
                <code className="font-mono text-sm font-medium text-fg">{c.command}</code>
                <span className="mt-1.5 block text-sm leading-relaxed text-muted">{c.purpose}</span>
              </button>
            </li>
          ))}
        </ul>

        <div className="min-w-0">
          <div className="overflow-hidden rounded-2xl border border-code-line bg-code-bg">
            <WindowBar title="Terminal" />
            <div aria-live="polite">
              <TerminalText command={selected.command} output={output} className="min-h-[18rem]" />
            </div>
          </div>
          <div className="mt-4 rounded-xl border border-line bg-surface p-4">
            <p className="text-sm font-semibold">Exit codes</p>
            <dl className="mt-3 grid gap-2 text-sm sm:grid-cols-2">
              {EXIT_CODES.map((e) => (
                <div key={e.code} className="flex gap-3">
                  <dt className="font-mono font-medium text-accent">{e.code}</dt>
                  <dd className="text-muted">{e.meaning}</dd>
                </div>
              ))}
            </dl>
          </div>
        </div>
      </Reveal>
    </Section>
  );
}
