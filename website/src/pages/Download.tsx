import { useState, useSyncExternalStore, type KeyboardEvent } from "react";
import {
  Boxes,
  CircleCheck,
  Clock,
  Cpu,
  Download as DownloadIcon,
  ExternalLink,
  FileText,
  Laptop,
  Monitor,
  Package,
  Play,
  Server,
  ShieldCheck,
  Terminal,
} from "lucide-react";
import {
  INSTALLERS_CHECKSUMS_URL,
  INSTALLERS_MANIFEST_URL,
  INSTALLERS_RELEASED,
  INSTALLERS_RELEASE_URL,
  INSTALLERS_VERSION,
  INSTALL_PS1,
  INSTALL_SH,
  INSTALL_SH_RUNTIME,
  OS_DOWNLOADS,
  detectOs,
  downloadUrl,
  type Artifact,
  type Edition,
  type OsDownloads,
  type OsId,
} from "../data/downloads";
import { RELEASED, VERSION, releaseAnchor, readmeSection } from "../data/site";
import { cn } from "../lib/cn";
import { Card, IconBadge, Pill } from "../components/ui/Card";
import { CopyButton, PromptButton } from "../components/ui/CodeBlock";
import { commandPrompt } from "../lib/aiPrompt";
import { Container } from "../components/ui/SectionHeading";

const OS_ICONS: Record<OsId, typeof Laptop> = { macos: Laptop, windows: Monitor, linux: Server };

const EDITIONS: Record<Edition, { name: string; tagline: string; icon: typeof Boxes; includes: string[]; commands: string }> = {
  sdk: {
    name: "Tread SDK",
    tagline: "For writing Tread programs. Everything in one tread command: compiler, bytecode tools, and the runtime.",
    icon: Boxes,
    includes: [
      "Compiler: lexer, parser, type checker, bytecode compiler",
      "Tread Virtual Machine (the runtime is built in)",
      "Project tool and installation diagnostics",
    ],
    commands: "new · check · build · run · doctor",
  },
  runtime: {
    name: "Tread Runtime",
    tagline: "For running programs someone already compiled. Smaller, and contains no compiler.",
    icon: Cpu,
    includes: ["Tread Virtual Machine", "Bytecode loader and verifier for .tbc files", "Installation diagnostics"],
    commands: "run <file.tbc> · doctor",
  },
};

const SHELLS: Record<OsId, string> = {
  macos: "a macOS terminal (zsh or bash)",
  linux: "a Linux terminal (bash or sh)",
  windows: "Windows PowerShell",
};

function Command({ text, label, prompt }: { text: string; label: string; prompt: string }) {
  return (
    <div className="flex items-center gap-2 rounded-xl border border-code-line bg-code-bg p-2 pl-4">
      <code className="scrollbar-thin min-w-0 flex-1 overflow-x-auto py-1 font-mono text-[12.5px] whitespace-nowrap text-code-fg">
        {text}
      </code>
      <PromptButton prompt={prompt} label={label.replace(/^Copy/, "Copy an AI prompt for")} className="shrink-0" />
      <CopyButton text={text} label={label} className="shrink-0" />
    </div>
  );
}

// installPrompt asks an AI assistant to run one of the install scripts.
function installPrompt(os: OsDownloads, edition: Edition, command: string): string {
  const name = edition === "sdk" ? "Tread SDK" : "Tread Runtime";
  const home = os.id === "windows" ? "%USERPROFILE%\\.tread" : "~/.tread";
  return commandPrompt({
    goal: `install the ${name} (the Tread programming language) on ${os.name}, for my user only`,
    command,
    shell: SHELLS[os.id],
    about: `It downloads the latest ${name} release from GitHub, verifies it against the release's SHA256SUMS, installs it into ${home} and adds it to my PATH; it needs no administrator rights.`,
    check: "open a new terminal so the PATH change applies, run `tread doctor` and `tread version`",
  });
}

function ArtifactRow({ a, os }: { a: Artifact; os: OsDownloads }) {
  const edition = a.edition === "sdk" ? "SDK" : "Runtime";
  const body = (
    <>
      <span className="min-w-0">
        <span className="flex flex-wrap items-center gap-2 text-sm font-medium">
          {a.label}
          <span className="rounded-md border border-line px-1.5 py-0.5 font-mono text-[10.5px] font-normal text-subtle">{a.format}</span>
        </span>
        <span className="mt-0.5 block truncate font-mono text-[11px] text-subtle">{a.file}</span>
      </span>
      {INSTALLERS_RELEASED ? (
        <DownloadIcon className="size-4 shrink-0 text-muted transition-colors group-hover:text-accent" aria-hidden />
      ) : (
        <span className="shrink-0 text-[11px] text-subtle">Not yet published</span>
      )}
    </>
  );
  return (
    <li>
      {INSTALLERS_RELEASED ? (
        <a
          href={downloadUrl(a.file)}
          className="group flex items-center justify-between gap-3 rounded-xl border border-line bg-surface-2/60 px-4 py-3 transition-colors hover:border-accent/60 hover:bg-accent-soft"
          aria-label={`Download Tread ${edition} ${INSTALLERS_VERSION} for ${os.name}, ${a.label} (${a.format}): ${a.file}`}
        >
          {body}
        </a>
      ) : (
        <div className="flex items-center justify-between gap-3 rounded-xl border border-dashed border-line px-4 py-3">{body}</div>
      )}
    </li>
  );
}

function ArtifactGroup({ title, hint, items, os }: { title: string; hint: string; items: Artifact[]; os: OsDownloads }) {
  if (items.length === 0) return null;
  return (
    <div>
      <h4 className="text-sm font-semibold">{title}</h4>
      <p className="mt-1 text-xs text-subtle">{hint}</p>
      <ul className="mt-3 flex flex-col gap-2">
        {items.map((a) => (
          <ArtifactRow key={a.file} a={a} os={os} />
        ))}
      </ul>
    </div>
  );
}

function OsPanel({ os, hidden }: { os: OsDownloads; hidden: boolean }) {
  const installers = os.artifacts.filter((a) => a.kind === "installer");
  const sdk = os.artifacts.filter((a) => a.kind === "archive" && a.edition === "sdk");
  const runtime = os.artifacts.filter((a) => a.kind === "archive" && a.edition === "runtime");
  const unix = os.id !== "windows";
  return (
    <div role="tabpanel" id={`panel-${os.id}`} aria-labelledby={`tab-${os.id}`} hidden={hidden} className="mt-6">
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)]">
        <Card>
          <div className="flex items-center gap-3">
            <IconBadge>
              <Package className="size-5" />
            </IconBadge>
            <div>
              <h3 className="font-semibold">Tread SDK installer for {os.name}</h3>
              <p className="text-xs text-subtle">{os.requirement} · recommended</p>
            </div>
          </div>
          <ul className="mt-5 flex flex-col gap-2">
            {installers.map((a) => (
              <ArtifactRow key={a.file} a={a} os={os} />
            ))}
          </ul>
          <dl className="mt-5 grid grid-cols-1 gap-3 border-t border-line pt-4 text-sm sm:grid-cols-[8rem_minmax(0,1fr)]">
            <dt className="text-subtle">Installs to</dt>
            <dd className="font-mono text-[12.5px] break-all">{os.installsTo}</dd>
            <dt className="text-subtle">PATH</dt>
            <dd className="text-muted">{os.path}</dd>
            <dt className="text-subtle">Uninstall</dt>
            <dd className="font-mono text-[12.5px] break-all">{os.uninstall}</dd>
          </dl>
          {os.note && <p className="mt-4 text-xs leading-relaxed text-subtle">{os.note}</p>}
        </Card>

        <Card>
          <div className="flex items-center gap-3">
            <IconBadge>
              <Terminal className="size-5" />
            </IconBadge>
            <div>
              <h3 className="font-semibold">Install from the terminal</h3>
              <p className="text-xs text-subtle">For your user only · no administrator rights</p>
            </div>
          </div>
          <p className="mt-4 text-sm leading-relaxed text-muted">
            Downloads the archive, checks it against <code className="font-mono text-[13px]">SHA256SUMS</code>, installs into{" "}
            <code className="font-mono text-[13px]">{unix ? "~/.tread" : "%USERPROFILE%\\.tread"}</code>, and adds it to your PATH
            once. Run it again to upgrade.
          </p>
          <div className="mt-4 flex flex-col gap-2">
            <p className="text-xs font-medium text-muted">Tread SDK</p>
            <Command
              text={unix ? INSTALL_SH : INSTALL_PS1}
              label={`Copy the ${os.name} SDK install command`}
              prompt={installPrompt(os, "sdk", unix ? INSTALL_SH : INSTALL_PS1)}
            />
            {unix && (
              <>
                <p className="mt-2 text-xs font-medium text-muted">Tread Runtime only</p>
                <Command
                  text={INSTALL_SH_RUNTIME}
                  label={`Copy the ${os.name} Runtime install command`}
                  prompt={installPrompt(os, "runtime", INSTALL_SH_RUNTIME)}
                />
              </>
            )}
          </div>
          <p className="mt-4 text-xs text-subtle">
            Uninstall:{" "}
            <code className="font-mono">{unix ? "sh install.sh --uninstall" : "install.ps1 -Uninstall"}</code>
            {!unix && (
              <>
                . Runtime only: download <code className="font-mono">install.ps1</code> and run it with{" "}
                <code className="font-mono">-Runtime</code>
              </>
            )}
            .
          </p>
        </Card>
      </div>

      <Card className="mt-4">
        <h3 className="font-semibold">Archives</h3>
        <p className="mt-1 text-sm text-muted">
          Unpack anywhere and add its <code className="font-mono text-[13px]">bin</code> folder to your PATH. Each archive holds{" "}
          <code className="font-mono text-[13px]">bin/tread</code>, <code className="font-mono text-[13px]">docs/</code> and a{" "}
          <code className="font-mono text-[13px]">release</code> file that <code className="font-mono text-[13px]">tread doctor</code>{" "}
          checks.
        </p>
        <div className="mt-5 grid grid-cols-1 gap-6 lg:grid-cols-2">
          <ArtifactGroup title="Tread SDK" hint="Compiler, tools and runtime." items={sdk} os={os} />
          <ArtifactGroup title="Tread Runtime" hint="Runs compiled .tbc programs; no compiler." items={runtime} os={os} />
        </div>
        <div className="mt-5 border-t border-line pt-4">
          <p className="text-xs font-medium text-muted">Verify a download</p>
          <code className="mt-1.5 block font-mono text-[11px] break-all text-subtle">{os.verify}</code>
        </div>
      </Card>
    </div>
  );
}

// The operating system never changes while the page is open.
const noSubscribe = () => () => {};

function PlatformTabs() {
  // Prerendered with macOS selected (the server snapshot is null); in the
  // browser, the visitor's operating system is selected until they pick.
  const detected = useSyncExternalStore(noSubscribe, detectOs, () => null);
  const [choice, setSelected] = useState<OsId | null>(null);
  const selected: OsId = choice ?? detected ?? "macos";

  const ids = OS_DOWNLOADS.map((o) => o.id);
  const onKey = (e: KeyboardEvent<HTMLButtonElement>) => {
    const i = ids.indexOf(selected);
    let next: number | null = null;
    if (e.key === "ArrowRight") next = (i + 1) % ids.length;
    if (e.key === "ArrowLeft") next = (i - 1 + ids.length) % ids.length;
    if (e.key === "Home") next = 0;
    if (e.key === "End") next = ids.length - 1;
    if (next === null) return;
    e.preventDefault();
    setSelected(ids[next]!);
    document.getElementById(`tab-${ids[next]}`)?.focus();
  };

  return (
    <div>
      <div role="tablist" aria-label="Operating system" className="inline-flex flex-wrap gap-1 rounded-xl border border-line bg-surface p-1">
        {OS_DOWNLOADS.map((os) => {
          const Icon = OS_ICONS[os.id];
          const active = os.id === selected;
          return (
            <button
              key={os.id}
              id={`tab-${os.id}`}
              type="button"
              role="tab"
              aria-selected={active}
              aria-controls={`panel-${os.id}`}
              tabIndex={active ? 0 : -1}
              onClick={() => setSelected(os.id)}
              onKeyDown={onKey}
              className={cn(
                "inline-flex h-10 items-center gap-2 rounded-lg px-4 text-sm font-medium transition-colors",
                active ? "bg-accent-soft text-fg shadow-[inset_0_0_0_1px_var(--accent)]" : "text-muted hover:bg-surface-2 hover:text-fg",
              )}
            >
              <Icon className="size-4" aria-hidden />
              {os.name}
              {detected === os.id && <span className="text-[11px] font-normal text-accent">· your system</span>}
            </button>
          );
        })}
      </div>
      {OS_DOWNLOADS.map((os) => (
        <OsPanel key={os.id} os={os} hidden={os.id !== selected} />
      ))}
    </div>
  );
}

export function DownloadPage() {
  return (
    <>
      <section aria-labelledby="download-page-title" className="border-b border-line">
        <Container className="pt-14 pb-12 sm:pt-20">
          <p className="font-mono text-xs font-medium tracking-[0.18em] text-accent uppercase">Download · v{INSTALLERS_VERSION}</p>
          <h1 id="download-page-title" className="mt-3 text-4xl font-semibold tracking-tight sm:text-5xl">
            Download Tread
          </h1>
          <p className="mt-4 max-w-2xl text-base leading-relaxed text-pretty text-muted sm:text-lg">
            Install the Tread SDK to write and run Tread programs, or the Tread Runtime to run programs that are already
            compiled. Installers set up your PATH for you, and you do not need Go.
          </p>

          {INSTALLERS_RELEASED ? (
            <p className="mt-6 inline-flex items-start gap-2.5 rounded-xl border border-accent/30 bg-accent-soft px-4 py-3 text-sm text-fg">
              <CircleCheck className="mt-0.5 size-4 shrink-0 text-accent" aria-hidden />
              Tread {INSTALLERS_VERSION} installers and archives are available for Windows, macOS and Linux.
            </p>
          ) : (
            <div className="mt-6 max-w-3xl rounded-2xl border border-accent/30 bg-accent-soft p-5">
              <p className="flex items-start gap-2.5 text-sm text-fg">
                <Clock className="mt-0.5 size-4 shrink-0 text-accent" aria-hidden />
                <span>
                  <strong className="font-semibold">The SDK, Runtime and installers arrive with Tread {INSTALLERS_VERSION}</strong>, which
                  has not been published yet. Every file below is built and tested on its own platform in CI, and the links
                  will work on release day.{" "}
                  {RELEASED ? (
                    <>
                      Today, download{" "}
                      <a href="/#download" className="font-medium text-accent hover:underline">
                        Tread {VERSION}
                      </a>
                      .
                    </>
                  ) : null}{" "}
                  <a href={releaseAnchor("Unreleased")} className="text-accent hover:underline">
                    What&rsquo;s coming
                  </a>
                  .
                </span>
              </p>
            </div>
          )}
        </Container>
      </section>

      <Container className="py-12 sm:py-16">
        <section aria-labelledby="editions-title">
          <h2 id="editions-title" className="text-2xl font-semibold tracking-tight">
            Choose a distribution
          </h2>
          <div className="mt-6 grid grid-cols-1 gap-4 md:grid-cols-2">
            {(Object.keys(EDITIONS) as Edition[]).map((id) => {
              const e = EDITIONS[id];
              const Icon = e.icon;
              return (
                <Card key={id} className="flex flex-col">
                  <div className="flex items-center gap-3">
                    <IconBadge>
                      <Icon className="size-5" />
                    </IconBadge>
                    <h3 className="text-lg font-semibold">{e.name}</h3>
                    {id === "sdk" && <Pill className="ml-auto">Most people want this</Pill>}
                  </div>
                  <p className="mt-3 text-sm leading-relaxed text-muted">{e.tagline}</p>
                  <ul className="mt-4 flex flex-1 flex-col gap-2 text-sm">
                    {e.includes.map((x) => (
                      <li key={x} className="flex gap-2.5">
                        <CircleCheck className="mt-0.5 size-4 shrink-0 text-accent" aria-hidden />
                        {x}
                      </li>
                    ))}
                  </ul>
                  <p className="mt-4 border-t border-line pt-3 font-mono text-xs text-subtle">tread {e.commands}</p>
                </Card>
              );
            })}
          </div>
        </section>

        <section aria-labelledby="platforms-title" className="mt-16">
          <h2 id="platforms-title" className="text-2xl font-semibold tracking-tight">
            Choose your platform
          </h2>
          <p className="mt-2 max-w-2xl text-sm text-muted">
            Every installer and archive is built from the same source, and each one is installed, upgraded and uninstalled on its
            own operating system in CI before a release.
          </p>
          <div className="mt-6">
            <PlatformTabs />
          </div>
        </section>

        <section aria-labelledby="after-title" className="mt-16 grid grid-cols-1 gap-4 lg:grid-cols-2">
          <Card>
            <div className="flex items-center gap-3">
              <IconBadge>
                <Play className="size-5" />
              </IconBadge>
              <h2 id="after-title" className="font-semibold">
                After installing
              </h2>
            </div>
            <p className="mt-3 text-sm text-muted">Open a new terminal, then:</p>
            <pre className="scrollbar-thin mt-3 overflow-x-auto rounded-xl border border-code-line bg-code-bg p-4 font-mono text-[12.5px] leading-6 text-code-fg">
              <code>
                {[
                  "tread --version          # tread " + INSTALLERS_VERSION,
                  "tread doctor             # checks PATH, TREAD_HOME and the runtime",
                  "tread new hello && cd hello",
                  "tread run src/main.tr    # Hello from hello!",
                  "tread build src/main.tr  # writes src/main.tbc",
                ].join("\n")}
              </code>
            </pre>
            <p className="mt-3 text-xs text-subtle">
              A <code className="font-mono">.tbc</code> file runs on the Tread Runtime of the same release. Rebuild it when you
              upgrade: bytecode is not yet compatible across releases.
            </p>
          </Card>

          <Card>
            <div className="flex items-center gap-3">
              <IconBadge>
                <ShieldCheck className="size-5" />
              </IconBadge>
              <h2 className="font-semibold">Verify and trust</h2>
            </div>
            <p className="mt-3 text-sm leading-relaxed text-muted">
              Check every download against <code className="font-mono text-[13px]">SHA256SUMS</code> (the install scripts do it for
              you). The installers and binaries are not code-signed yet, so macOS Gatekeeper and Windows SmartScreen may warn the
              first time.
            </p>
            <ul className="mt-4 flex flex-col gap-2 text-sm">
              {INSTALLERS_RELEASED && (
                <>
                  <li>
                    <a href={INSTALLERS_CHECKSUMS_URL} className="inline-flex items-center gap-2 text-accent hover:underline">
                      <FileText className="size-4" aria-hidden />
                      SHA256SUMS
                    </a>
                  </li>
                  <li>
                    <a href={INSTALLERS_MANIFEST_URL} className="inline-flex items-center gap-2 text-accent hover:underline">
                      <FileText className="size-4" aria-hidden />
                      release-manifest.json
                    </a>
                    <span className="text-subtle">: every file with its size and SHA-256</span>
                  </li>
                  <li>
                    <a href={INSTALLERS_RELEASE_URL} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-2 text-accent hover:underline">
                      <ExternalLink className="size-4" aria-hidden />
                      Release {INSTALLERS_VERSION} on GitHub
                      <span className="sr-only">(opens in a new tab)</span>
                    </a>
                  </li>
                </>
              )}
              <li>
                <a href={readmeSection("installation")} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-2 text-accent hover:underline">
                  <ExternalLink className="size-4" aria-hidden />
                  Installation guide (README)
                  <span className="sr-only">(opens in a new tab)</span>
                </a>
              </li>
            </ul>
          </Card>
        </section>
      </Container>
    </>
  );
}
