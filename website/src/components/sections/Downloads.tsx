import { CircleCheck, Download, ExternalLink, FileText, Laptop, Monitor, Server, ShieldCheck } from "lucide-react";
import {
  CHECKSUMS_URL,
  GO_INSTALL,
  PLATFORMS,
  RELEASE_DATE,
  RELEASE_URL,
  RELEASES_URL,
  VERSION,
  asset,
  readmeSection,
  type Platform,
} from "../../data/site";
import { Card, IconBadge } from "../ui/Card";
import { CopyButton } from "../ui/CodeBlock";
import { Reveal } from "../ui/Reveal";
import { Section, SectionHeading } from "../ui/SectionHeading";

const ICONS: Record<Platform["id"], typeof Laptop> = { macos: Laptop, linux: Server, windows: Monitor };

function PlatformCard({ platform }: { platform: Platform }) {
  const Icon = ICONS[platform.id];
  return (
    <Card className="flex h-full flex-col">
      <div className="flex items-center gap-3">
        <IconBadge>
          <Icon className="size-5" />
        </IconBadge>
        <div>
          <h3 className="font-semibold">{platform.name}</h3>
          <p className="text-xs text-subtle">{platform.requirement}</p>
        </div>
      </div>

      <ul className="mt-6 flex flex-1 flex-col gap-2">
        {platform.builds.map((b) => (
          <li key={b.file}>
            <a
              href={asset(b.file)}
              className="group flex items-center justify-between gap-3 rounded-xl border border-line bg-surface-2/60 px-4 py-3 transition-colors hover:border-accent/60 hover:bg-accent-soft"
              aria-label={`Download AURA ${VERSION} for ${platform.name} ${b.label} (${b.arch}): ${b.file}`}
            >
              <span className="min-w-0">
                <span className="block text-sm font-medium">
                  {b.label} <span className="font-normal text-subtle">· {b.arch}</span>
                </span>
                <span className="block truncate font-mono text-[11px] text-subtle">{b.file}</span>
              </span>
              <Download className="size-4 shrink-0 text-muted transition-colors group-hover:text-accent" aria-hidden />
            </a>
          </li>
        ))}
      </ul>

      <div className="mt-5 border-t border-line pt-4">
        <p className="text-xs font-medium text-muted">Verify the download</p>
        <code className="mt-1.5 block font-mono text-[11px] break-all text-subtle">{platform.verify}</code>
      </div>
    </Card>
  );
}

export function Downloads() {
  return (
    <Section id="download" labelledBy="download-title" className="border-y border-line bg-surface-2/40">
      <SectionHeading
        id="download-title"
        eyebrow={`Download · v${VERSION}`}
        title="Start building with AURA."
        description={`Get started with AURA ${VERSION}. Download a precompiled release for your operating system and run your first AURA program.`}
      />

      <Reveal className="mt-8">
        <p className="inline-flex items-start gap-2.5 rounded-xl border border-accent/30 bg-accent-soft px-4 py-3 text-sm text-fg">
          <CircleCheck className="mt-0.5 size-4 shrink-0 text-accent" aria-hidden />
          Precompiled binaries are available. You do not need Go installed to download and run AURA.
        </p>
      </Reveal>

      <div className="mt-8 grid grid-cols-1 gap-4 md:grid-cols-3">
        {PLATFORMS.map((p, i) => (
          <Reveal key={p.id} delay={0.05 * i} className="h-full">
            <PlatformCard platform={p} />
          </Reveal>
        ))}
      </div>

      <Reveal className="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)]">
        <Card>
          <div className="flex items-center gap-3">
            <IconBadge>
              <ShieldCheck className="size-5" />
            </IconBadge>
            <h3 className="font-semibold">Release {VERSION}</h3>
          </div>
          <p className="mt-3 text-sm leading-relaxed text-muted">
            Released {RELEASE_DATE}. Each archive contains the <code className="font-mono text-[13px]">aura</code>{" "}
            executable, LICENSE, README, CHANGELOG, and third-party notices. The binaries are not code-signed.
          </p>
          <ul className="mt-4 flex flex-col gap-2 text-sm">
            <li>
              <a href={CHECKSUMS_URL} className="inline-flex items-center gap-2 text-accent hover:underline">
                <FileText className="size-4" aria-hidden />
                SHA256SUMS
              </a>
              <span className="text-subtle">: checksums for all five archives</span>
            </li>
            <li>
              <a href={RELEASE_URL} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-2 text-accent hover:underline">
                <ExternalLink className="size-4" aria-hidden />
                Release notes on GitHub
                <span className="sr-only">(opens in a new tab)</span>
              </a>
            </li>
            <li>
              <a href={readmeSection("installation")} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-2 text-accent hover:underline">
                <ExternalLink className="size-4" aria-hidden />
                Step-by-step installation guide
                <span className="sr-only">(opens in a new tab)</span>
              </a>
            </li>
            <li>
              <a href={RELEASES_URL} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-2 text-muted hover:text-fg hover:underline">
                <ExternalLink className="size-4" aria-hidden />
                All releases, including 0.1.0
                <span className="sr-only">(opens in a new tab)</span>
              </a>
            </li>
          </ul>
        </Card>

        <Card>
          <h3 className="font-semibold">For Go developers</h3>
          <p className="mt-2 text-sm leading-relaxed text-muted">
            Prefer to install from source? This method <strong className="font-semibold text-fg">requires Go 1.25 or later</strong>.
            The <code className="font-mono text-[13px]">aura</code> binary is placed in{" "}
            <code className="font-mono text-[13px]">$(go env GOPATH)/bin</code>, which must be on your PATH.
          </p>
          <div className="mt-4 flex items-center gap-2 rounded-xl border border-code-line bg-code-bg p-2 pl-4">
            <code className="scrollbar-thin min-w-0 flex-1 overflow-x-auto py-1 font-mono text-[12.5px] whitespace-nowrap text-code-fg">
              {GO_INSTALL}
            </code>
            <CopyButton text={GO_INSTALL} label="Copy the go install command" className="shrink-0" />
          </div>
        </Card>
      </Reveal>
    </Section>
  );
}
