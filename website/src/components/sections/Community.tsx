import { ArrowUpRight, Bug, Code2, GitPullRequest, Lightbulb } from "lucide-react";
import { ISSUES_URL, NEW_ISSUE_URL, REPO_URL, readmeSection } from "../../data/site";
import { ButtonLink } from "../ui/Button";
import { GitHubIcon } from "../ui/GitHubIcon";
import { Reveal } from "../ui/Reveal";
import { Section, SectionHeading } from "../ui/SectionHeading";

// AURA's only community channel today is its GitHub repository.
const ACTIONS = [
  { icon: Code2, title: "Explore the source", body: "Read the compiler, the VM, and their tests.", href: REPO_URL },
  { icon: Bug, title: "Report an issue", body: "Found a bug or a confusing error message? Tell us.", href: NEW_ISSUE_URL },
  { icon: Lightbulb, title: "Suggest a feature", body: "Propose an idea, or discuss an open one.", href: ISSUES_URL },
  { icon: GitPullRequest, title: "Contribute", body: "Read the contribution guide in the README.", href: readmeSection("contributing") },
];

export function Community() {
  return (
    <Section id="community" labelledBy="community-title">
      <div className="relative overflow-hidden rounded-3xl border border-line bg-surface px-6 py-12 sm:px-12 sm:py-16">
        <div
          className="pointer-events-none absolute -right-24 -bottom-32 h-80 w-80 rounded-full bg-accent/15 blur-[100px]"
          aria-hidden
        />
        <div className="relative grid grid-cols-1 gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)] lg:items-center">
          <div>
            <SectionHeading
              id="community-title"
              eyebrow="Open source"
              title="Built in the open, on GitHub."
              description="AURA is MIT-licensed and developed in public. Explore the code, report what breaks, and help shape what comes next."
            />
            <Reveal className="mt-8">
              <ButtonLink href={REPO_URL} size="lg">
                <GitHubIcon className="size-[18px]" />
                View the repository
                <span className="sr-only">(opens in a new tab)</span>
              </ButtonLink>
            </Reveal>
          </div>
          <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {ACTIONS.map((a, i) => (
              <li key={a.title}>
                <Reveal delay={0.04 * i} className="h-full">
                  <a
                    href={a.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="group flex h-full flex-col rounded-2xl border border-line bg-bg/60 p-5 transition-colors hover:border-line-strong"
                  >
                    <span className="flex items-center justify-between">
                      <a.icon className="size-5 text-accent" aria-hidden />
                      <ArrowUpRight className="size-4 text-subtle transition-colors group-hover:text-fg" aria-hidden />
                    </span>
                    <span className="mt-4 font-medium">{a.title}</span>
                    <span className="mt-1 text-sm text-muted">{a.body}</span>
                    <span className="sr-only">(opens on GitHub in a new tab)</span>
                  </a>
                </Reveal>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </Section>
  );
}
