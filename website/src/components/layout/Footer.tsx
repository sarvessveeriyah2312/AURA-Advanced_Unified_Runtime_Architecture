import { CHANGELOG_URL, LICENSE_URL, MOTTO, README_URL, RELEASED, REPO_URL, VERSION } from "../../data/site";
import { TreadMark } from "../ui/GitHubIcon";
import { Container } from "../ui/SectionHeading";

const LINKS = [
  { label: "GitHub", href: REPO_URL },
  { label: "Documentation", href: README_URL },
  { label: "Downloads", href: "#download" },
  { label: "License", href: LICENSE_URL },
  { label: "Changelog", href: CHANGELOG_URL },
];

export function Footer() {
  return (
    <footer className="border-t border-line">
      <Container className="flex flex-col gap-10 py-12 md:flex-row md:items-start md:justify-between">
        <div className="max-w-sm">
          <div className="flex items-center gap-2.5">
            <TreadMark className="size-8" />
            <p className="font-semibold">
              Tread
            </p>
          </div>
          <p className="mt-3 text-sm text-muted italic">{MOTTO}</p>
          <p className="mt-6 text-xs text-subtle">
            © 2026 Sarvess Veeriyah. Licensed under the MIT License. Version {VERSION}
            {RELEASED ? "" : " (unreleased)"}.
          </p>
          <p className="mt-2 text-xs text-subtle">
            A tread is one step of a staircase: Tread is learned one stage at a time, from lexer to parser, checker,
            compiler, and VM.
          </p>
        </div>
        <nav aria-label="Footer">
          <ul className="grid grid-cols-2 gap-x-10 gap-y-3 text-sm sm:grid-cols-3 md:grid-cols-1">
            {LINKS.map((l) => {
              const external = l.href.startsWith("http");
              return (
                <li key={l.label}>
                  <a
                    href={l.href}
                    className="text-muted transition-colors hover:text-fg"
                    {...(external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
                  >
                    {l.label}
                    {external && <span className="sr-only"> (opens in a new tab)</span>}
                  </a>
                </li>
              );
            })}
          </ul>
        </nav>
      </Container>
    </footer>
  );
}
