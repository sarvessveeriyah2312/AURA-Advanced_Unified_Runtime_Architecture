// The /download/ page: Tread SDK and Tread Runtime distributions and
// installers (ADR-0011). File names follow scripts/build-release.sh and
// the packaging/ scripts exactly; scripts/manifest rejects any other
// name, so a typo here would be a dead link.
import { REPO_URL, RELEASED, VERSION } from "./site";

// Every release from 0.4.0 on ships SDK/Runtime archives and installers,
// so the page always offers the site's current release: on release day
// set VERSION (and RELEASED/RELEASE_DATE) in site.ts, and this page
// follows.
export const INSTALLERS_VERSION: string = VERSION;
export const INSTALLERS_RELEASED = RELEASED;

const TAG = `v${INSTALLERS_VERSION}`;
const BASE = `${REPO_URL}/releases/download/${TAG}`;
export const INSTALLERS_RELEASE_URL = `${REPO_URL}/releases/tag/${TAG}`;
export const INSTALLERS_CHECKSUMS_URL = `${BASE}/SHA256SUMS`;
export const INSTALLERS_MANIFEST_URL = `${BASE}/release-manifest.json`;
export const downloadUrl = (file: string) => `${BASE}/${file}`;

// Always the newest release, so the one-line installers keep working
// across versions.
const LATEST = `${REPO_URL}/releases/latest/download`;
export const INSTALL_SH = `curl -fsSL ${LATEST}/install.sh | sh`;
export const INSTALL_SH_RUNTIME = `curl -fsSL ${LATEST}/install.sh | sh -s -- --runtime`;
export const INSTALL_PS1 = `powershell -NoProfile -ExecutionPolicy Bypass -Command "irm ${LATEST}/install.ps1 | iex"`;

export type Edition = "sdk" | "runtime";
export type OsId = "macos" | "windows" | "linux";
export type Kind = "installer" | "archive";

export interface Artifact {
  edition: Edition;
  kind: Kind;
  label: string; // e.g. "Apple silicon", "x86-64 · Debian, Ubuntu"
  format: string; // e.g. ".pkg"
  file: string;
}

export interface OsDownloads {
  id: OsId;
  name: string;
  requirement: string;
  artifacts: Artifact[];
  installsTo: string;
  path: string;
  uninstall: string;
  verify: string;
  note?: string;
}

const file = (edition: Edition, platform: string, ext: string) => `tread-${edition}-${INSTALLERS_VERSION}-${platform}.${ext}`;

function archives(platforms: { id: string; label: string }[], ext: string): Artifact[] {
  return (["sdk", "runtime"] as const).flatMap((edition) =>
    platforms.map((p) => ({ edition, kind: "archive" as const, label: p.label, format: `.${ext}`, file: file(edition, p.id, ext) })),
  );
}

export const OS_DOWNLOADS: OsDownloads[] = [
  {
    id: "macos",
    name: "macOS",
    requirement: "macOS 12 or later",
    artifacts: [
      { edition: "sdk", kind: "installer", label: "Apple silicon", format: ".pkg", file: file("sdk", "macos-arm64", "pkg") },
      { edition: "sdk", kind: "installer", label: "Intel", format: ".pkg", file: file("sdk", "macos-x64", "pkg") },
      ...archives(
        [
          { id: "macos-arm64", label: "Apple silicon" },
          { id: "macos-x64", label: "Intel" },
        ],
        "tar.gz",
      ),
    ],
    installsTo: "/Library/Tread",
    path: "/etc/paths.d/tread adds /Library/Tread/bin for new terminal windows",
    uninstall: "sudo /Library/Tread/uninstall.sh",
    verify: "shasum -a 256 -c SHA256SUMS --ignore-missing",
    note: "Not sure which Mac you have? Apple menu → About This Mac: “Chip: Apple M…” is Apple silicon; “Processor: Intel” is Intel.",
  },
  {
    id: "windows",
    name: "Windows",
    requirement: "Windows 10 / Server 2016 or later, x64",
    artifacts: [
      { edition: "sdk", kind: "installer", label: "x64 · per-user, no administrator rights", format: ".msi", file: file("sdk", "windows-x64", "msi") },
      ...archives([{ id: "windows-x64", label: "x64" }], "zip"),
    ],
    installsTo: "%LOCALAPPDATA%\\Programs\\Tread",
    path: "adds Tread's bin folder to your user PATH and sets TREAD_HOME",
    uninstall: "Settings → Apps → Installed apps → Tread SDK → Uninstall",
    verify: "Get-FileHash .\\<file> -Algorithm SHA256",
  },
  {
    id: "linux",
    name: "Linux",
    requirement: "Kernel 3.2 or later · static binaries",
    artifacts: [
      { edition: "sdk", kind: "installer", label: "x86-64 · Debian, Ubuntu", format: ".deb", file: file("sdk", "linux-x64", "deb") },
      { edition: "sdk", kind: "installer", label: "ARM64 · Debian, Ubuntu", format: ".deb", file: file("sdk", "linux-arm64", "deb") },
      { edition: "sdk", kind: "installer", label: "x86-64 · Fedora, RHEL, openSUSE", format: ".rpm", file: file("sdk", "linux-x64", "rpm") },
      { edition: "sdk", kind: "installer", label: "ARM64 · Fedora, RHEL, openSUSE", format: ".rpm", file: file("sdk", "linux-arm64", "rpm") },
      ...archives(
        [
          { id: "linux-x64", label: "x86-64" },
          { id: "linux-arm64", label: "ARM64" },
        ],
        "tar.gz",
      ),
    ],
    installsTo: "/opt/tread",
    path: "links /usr/bin/tread",
    uninstall: "sudo apt remove tread   ·   sudo dnf remove tread",
    verify: "sha256sum -c SHA256SUMS --ignore-missing",
  },
];

// Detects the visitor's operating system in the browser (never during
// prerendering). Returns null when unsure.
export function detectOs(): OsId | null {
  if (typeof navigator === "undefined") return null;
  const nav = navigator as Navigator & { userAgentData?: { platform?: string } };
  const p = `${nav.userAgentData?.platform ?? ""} ${nav.platform ?? ""} ${nav.userAgent}`.toLowerCase();
  if (/iphone|ipad|android/.test(p)) return null; // no Tread builds for phones
  if (p.includes("win")) return "windows";
  if (p.includes("mac")) return "macos";
  if (p.includes("linux") || p.includes("x11")) return "linux";
  return null;
}
