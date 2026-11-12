import { ArrowRight, CircleCheck, Clock, Download, ExternalLink, FileText, Laptop, Monitor, Package, Server, ShieldCheck } from "lucide-react";
import { INSTALLERS_RELEASED, INSTALLERS_VERSION } from "../../data/downloads";
import {
  BUILD_FROM_SOURCE,
  CHECKSUMS_URL,
  DOWNLOAD_PAGE,
  GO_INSTALL,
  PLATFORMS,
  RELEASED,
  RELEASE_DATE,
  RELEASE_URL,
  RELEASES_URL,
  VERSION,
  asset,
  releaseAnchor,
  readmeSection,
  type Platform,
} from "../../data/site";
import { Card, IconBadge } from "../ui/Card";
import { CopyButton } from "../ui/CodeBlock";
import { Reveal } from "../ui/Reveal";
import { Section, SectionHeading } from "../ui/SectionHeading";

const ICONS: Record<Platform["id"], typeof Laptop> = { macos: Laptop, linux: Server, windows: Monitor };

function BuildLabel({ label, arch, file }: { label: string; arch: string; file: string }) {
  return (
    <span className="min-w-0">
      <span className="block text-sm font-medium">
        {label} <span className="font-normal text-subtle">· {arch}</span>
      </span>
      <span className="block truncate font-mono text-[11px] text-subtle">{file}</span>
    </span>
  );
}

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
            {RELEASED ? (
              <a
                href={asset(b.file)}
                className="group flex items-center justify-between gap-3 rounded-xl border border-line bg-surface-2/60 px-4 py-3 transition-colors hover:border-accent/60 hover:bg-accent-soft"
                aria-label={`Download Tread ${VERSION} for ${platform.name} ${b.label} (${b.arch}): ${b.file}`}
              >
                <BuildLabel label={b.label} arch={b.arch} file={b.file} />
                <Download className="size-4 shrink-0 text-muted transition-colors group-hover:text-accent" aria-hidden />
              </a>
            ) : (
              <div className="flex items-center justify-between gap-3 rounded-xl border border-dashed border-line px-4 py-3">
                <BuildLabel label={b.label} arch={b.arch} file={b.file} />
                <span className="shrink-0 text-[11px] text-subtle">Not yet published</span>
              </div>
            )}
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
        title="Start building with Tread."
        description={
          RELEASED
            ? `Get started with Tread ${VERSION}. Download a precompiled release for your operating system and run your first Tread program.`
            : `Tread ${VERSION} is on its way. Until its release is published, build Tread from source and run your first program in a minute.`
        }
      />

      <Reveal className="mt-8">
        {RELEASED ? (
          <p className="inline-flex items-start gap-2.5 rounded-xl border border-accent/30 bg-accent-soft px-4 py-3 text-sm text-fg">
            <CircleCheck className="mt-0.5 size-4 shrink-0 text-accent" aria-hidden />
            Precompiled binaries are available. You do not need Go installed to download and run Tread.
          </p>
        ) : (
          <div className="rounded-2xl border border-accent/30 bg-accent-soft p-5">
            <p className="flex items-start gap-2.5 text-sm text-fg">
              <Clock className="mt-0.5 size-4 shrink-0 text-accent" aria-hidden />
              <span>
                <strong className="font-semibold">Tread {VERSION} has not been published yet.</strong> Its precompiled
                archives, listed below, will not need Go. Today, build from source (Go 1.25 or later):
              </span>
            </p>
            <div className="mt-4 flex items-start gap-2 rounded-xl border border-code-line bg-code-bg p-2 pl-4">
              <pre className="scrollbar-thin min-w-0 flex-1 overflow-x-auto py-1 font-mono text-[12.5px] leading-6 text-code-fg">
                <code>{BUILD_FROM_SOURCE}</code>
              </pre>
              <CopyButton text={BUILD_FROM_SOURCE} label="Copy the build-from-source commands" className="shrink-0" />
            </div>
          </div>
        )}
      </Reveal>

      <Reveal className="mt-4">
        <a
          href={DOWNLOAD_PAGE}
          className="group flex flex-col gap-3 rounded-2xl border border-line bg-surface p-5 shadow-card transition-colors hover:border-accent/60 sm:flex-row sm:items-center"
        >
          <IconBadge>
            <Package className="size-5" />
          </IconBadge>
          <span className="min-w-0 flex-1">
            <span className="block font-semibold">
              Tread SDK, Tread Runtime and installers
              {!INSTALLERS_RELEASED && <span className="ml-2 text-xs font-normal text-subtle">coming in {INSTALLERS_VERSION}</span>}
            </span>
            <span className="mt-1 block text-sm text-muted">
              .msi for Windows, .pkg for macOS, .deb and .rpm for Linux, and one-line install scripts that set up your PATH.
            </span>
          </span>
          <span className="inline-flex items-center gap-1.5 text-sm font-medium text-accent">
            Download page
            <ArrowRight className="size-4 transition-transform group-hover:translate-x-0.5" aria-hidden />
          </span>
        </a>
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
            <h3 className="font-semibold">
              Release {VERSION}
              {!RELEASED && <span className="ml-2 text-xs font-normal text-subtle">not yet published</span>}
            </h3>
          </div>
          <p className="mt-3 text-sm leading-relaxed text-muted">
            {RELEASED && RELEASE_DATE ? `Released ${RELEASE_DATE}. ` : ""}Each archive contains the{" "}
            <code className="font-mono text-[13px]">tread</code> executable, LICENSE, README, CHANGELOG, and third-party
            notices. The binaries are not code-signed.
          </p>
          <ul className="mt-4 flex flex-col gap-2 text-sm">
            {RELEASED && (
              <li>
                <a href={CHECKSUMS_URL} className="inline-flex items-center gap-2 text-accent hover:underline">
                  <FileText className="size-4" aria-hidden />
                  SHA256SUMS
                </a>
                <span className="text-subtle">: checksums for all five archives</span>
              </li>
            )}
            <li>
              <a
                href={RELEASED ? RELEASE_URL : releaseAnchor(VERSION)}
                {...(RELEASED ? { target: "_blank", rel: "noopener noreferrer" } : {})}
                className="inline-flex items-center gap-2 text-accent hover:underline"
              >
                <ExternalLink className="size-4" aria-hidden />
                {RELEASED ? "Release notes on GitHub" : `What's in ${VERSION} (changelog)`}
                {RELEASED && <span className="sr-only">(opens in a new tab)</span>}
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
                All published releases
                <span className="sr-only">(opens in a new tab)</span>
              </a>
            </li>
          </ul>
        </Card>

        <Card>
          <h3 className="font-semibold">For Go developers</h3>
          <p className="mt-2 text-sm leading-relaxed text-muted">
            {!RELEASED && <>This command works once v{VERSION} is published. </>}
            Prefer to install with Go? This method <strong className="font-semibold text-fg">requires Go 1.25 or later</strong>.
            The <code className="font-mono text-[13px]">tread</code> binary is placed in{" "}
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
