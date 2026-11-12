import { ExternalLink } from "lucide-react";
import { CHANGELOG, type Block, type ListItem, type Release } from "../data/changelog";
import { CHANGELOG_URL } from "../data/site";
import { cn } from "../lib/cn";
import { InlineMarkdown } from "../components/ui/InlineMarkdown";
import { Container } from "../components/ui/SectionHeading";

function Items({ items, nested = false }: { items: ListItem[]; nested?: boolean }) {
  return (
    <ul className={cn("space-y-2", nested ? "mt-2 ml-1 border-l border-line pl-4" : "mt-3")}>
      {items.map((item, i) => (
        <li key={i} className="flex gap-3 text-[15px] leading-relaxed text-muted">
          {!nested && <span className="mt-[0.7em] size-1.5 shrink-0 rounded-full bg-accent/70" aria-hidden />}
          <div className="min-w-0">
            <InlineMarkdown text={item.text} />
            {item.children.length > 0 && <Items items={item.children} nested />}
          </div>
        </li>
      ))}
    </ul>
  );
}

function Blocks({ blocks }: { blocks: Block[] }) {
  return (
    <>
      {blocks.map((b, i) =>
        b.type === "p" ? (
          <p key={i} className="mt-3 text-[15px] leading-relaxed text-muted">
            <InlineMarkdown text={b.text} />
          </p>
        ) : (
          <Items key={i} items={b.items} />
        ),
      )}
    </>
  );
}

function formatDate(iso: string) {
  // Fixed locale and time zone, so build and browser render identical text.
  return new Date(iso + "T00:00:00Z").toLocaleDateString("en-GB", { day: "numeric", month: "long", year: "numeric", timeZone: "UTC" });
}

function ReleaseStatus({ release, latest }: { release: Release; latest: boolean }) {
  if (!release.date) {
    return <span className="rounded-full border border-accent/30 bg-accent-soft px-2.5 py-1 text-xs font-medium text-accent">Unreleased</span>;
  }
  return (
    <span className="flex flex-wrap items-center gap-2 text-sm text-subtle">
      <time dateTime={release.date}>{formatDate(release.date)}</time>
      {latest && (
        <span className="rounded-full border border-emerald-500/25 bg-emerald-500/10 px-2.5 py-1 text-xs font-medium text-emerald-700 dark:text-emerald-400">
          Latest release
        </span>
      )}
    </span>
  );
}

export function ChangelogPage() {
  const { releases } = CHANGELOG;
  const latest = releases.find((r) => r.date !== null);

  return (
    <>
      <section aria-labelledby="changelog-title" className="border-b border-line">
        <Container className="pt-14 pb-12 sm:pt-20">
          <p className="font-mono text-xs font-medium tracking-[0.18em] text-accent uppercase">Changelog</p>
          <h1 id="changelog-title" className="mt-3 text-4xl font-semibold tracking-tight sm:text-5xl">
            What&rsquo;s new in Tread
          </h1>
          <p className="mt-4 max-w-2xl text-base leading-relaxed text-pretty text-muted sm:text-lg">
            Every user-visible change, release by release, newest first. This page is generated from{" "}
            <code className="font-mono text-[0.9em]">CHANGELOG.md</code> in the repository, so it always matches the
            code.
          </p>
          <p className="mt-3 max-w-2xl text-sm text-subtle">
            Releases 0.1.0 and 0.2.0 were published under the project&rsquo;s previous name; their entries are kept
            as originally written.
          </p>
          <a
            href={CHANGELOG_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-5 inline-flex items-center gap-1.5 text-sm text-accent hover:underline"
          >
            <ExternalLink className="size-4" aria-hidden />
            View CHANGELOG.md on GitHub
            <span className="sr-only">(opens in a new tab)</span>
          </a>
        </Container>
      </section>

      <Container className="grid grid-cols-1 gap-10 py-12 sm:py-16 lg:grid-cols-[13rem_minmax(0,1fr)] lg:gap-16">
        <nav aria-label="Releases" className="lg:sticky lg:top-24 lg:self-start">
          <p className="text-xs font-medium tracking-wide text-subtle uppercase">Releases</p>
          <ul className="mt-3 flex flex-wrap gap-2 lg:flex-col lg:gap-1">
            {releases.map((r) => (
              <li key={r.id}>
                <a
                  href={`#${r.id}`}
                  className="flex items-baseline justify-between gap-4 rounded-lg border border-line px-3 py-2 text-sm transition-colors hover:border-line-strong hover:bg-surface-2 lg:border-transparent"
                >
                  <span className="font-mono font-medium text-fg">{r.version}</span>
                  <span className="text-xs text-subtle">{r.date ?? "Unreleased"}</span>
                </a>
              </li>
            ))}
          </ul>
        </nav>

        <ol className="min-w-0 space-y-14">
          {releases.map((r) => (
            <li key={r.id}>
              <article id={r.id} aria-labelledby={`${r.id}-title`} className="scroll-mt-24">
                <header className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-line pb-4">
                  <h2 id={`${r.id}-title`} className="font-mono text-2xl font-semibold tracking-tight">
                    {r.version}
                  </h2>
                  <ReleaseStatus release={r} latest={r === latest} />
                </header>
                <Blocks blocks={r.intro} />
                {r.sections.map((s) => (
                  <section key={s.title} className="mt-8">
                    <h3 className="text-sm font-semibold tracking-wide text-fg uppercase">{s.title}</h3>
                    <Blocks blocks={s.blocks} />
                  </section>
                ))}
              </article>
            </li>
          ))}
        </ol>
      </Container>
    </>
  );
}
