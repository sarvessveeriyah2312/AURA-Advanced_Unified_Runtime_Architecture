import { ArrowUpRight, BookMarked, BookOpen, Bug, Code2, FileText, History, ListTree, Rocket, SquareTerminal } from "lucide-react";
import { CHANGELOG_URL, ISSUES_URL, README_URL, SOURCE_URL, readmeSection } from "../../data/site";
import { Reveal } from "../ui/Reveal";
import { Section, SectionHeading } from "../ui/SectionHeading";

// Every link points at documentation that exists in the public
// repository (verified). AURA's design records are not published yet,
// so the README's language reference is the public reference.
const DOCS = [
  { icon: BookOpen, title: "README", body: "Overview, installation, and the full guide.", href: README_URL },
  { icon: Rocket, title: "Installation", body: "Release downloads, go install, and building from source.", href: readmeSection("installation") },
  { icon: BookMarked, title: "Language tutorial", body: "Eleven short lessons, each a complete program.", href: readmeSection("language-tutorial") },
  { icon: ListTree, title: "Language reference", body: "Types, operators, keywords, and scope rules.", href: readmeSection("language-reference") },
  { icon: SquareTerminal, title: "CLI reference", body: "Commands, behaviour, and exit codes.", href: readmeSection("cli-reference") },
  { icon: FileText, title: "Errors and diagnostics", body: "Error format, code families, and stack traces.", href: readmeSection("errors-and-diagnostics") },
  { icon: History, title: "Changelog", body: "Every user-visible change, release by release.", href: CHANGELOG_URL },
  { icon: Code2, title: "Source code", body: "Compiler, virtual machine, CLI, and tests, in Go.", href: SOURCE_URL },
  { icon: Bug, title: "Issues", body: "Bug reports and feature requests.", href: ISSUES_URL },
];

export function Documentation() {
  return (
    <Section id="documentation" labelledBy="documentation-title">
      <SectionHeading
        id="documentation-title"
        eyebrow="Documentation"
        title="Everything you need to learn AURA."
        description="AURA's documentation lives with its source on GitHub, so it always matches the code."
      />
      <ul className="mt-12 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {DOCS.map((d, i) => (
          <li key={d.title}>
            <Reveal delay={0.03 * (i % 3)} className="h-full">
              <a
                href={d.href}
                target="_blank"
                rel="noopener noreferrer"
                className="group flex h-full items-start gap-4 rounded-2xl border border-line bg-surface p-5 transition-colors hover:border-line-strong"
              >
                <d.icon className="mt-0.5 size-5 shrink-0 text-accent" aria-hidden />
                <span className="min-w-0 flex-1">
                  <span className="flex items-center justify-between gap-2 font-medium">
                    {d.title}
                    <ArrowUpRight
                      className="size-4 shrink-0 text-subtle transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5 group-hover:text-fg"
                      aria-hidden
                    />
                  </span>
                  <span className="mt-1 block text-sm text-muted">{d.body}</span>
                  <span className="sr-only">(opens on GitHub in a new tab)</span>
                </span>
              </a>
            </Reveal>
          </li>
        ))}
      </ul>
    </Section>
  );
}
