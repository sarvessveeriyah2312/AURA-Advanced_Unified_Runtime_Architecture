// Single source of truth for Tread facts shown on the site. Every value
// here was verified against the repository and the published GitHub
// release (see the README in this folder). To move the site to a new
// release, update VERSION and confirm the asset list still matches.

export const VERSION = "0.3.0";
// RELEASED is false until the v0.3.0 GitHub release exists. While false,
// the site shows the upcoming archive names without download links and
// points visitors at building from source instead. Set it to true (and
// RELEASE_DATE) on release day.
export const RELEASED = false;
export const RELEASE_DATE: string | null = null;
export const MOTTO = "A language for learning how languages work.";

// Canonical address of this website. index.html, public/robots.txt and
// public/llms.txt repeat it as literals (they are static files); change
// all of them together if the domain changes.
export const SITE_URL = "https://treadlang.vercel.app/";
// Shown in the footer and used as the sitemap's <lastmod>.
export const UPDATED = "2026-11-12";
export const AUTHOR = { name: "Sarvess Veeriyah", url: "https://github.com/sarvessveeriyah2312" };

export const OWNER = "sarvessveeriyah2312";
export const REPO_NAME = "AURA-Advanced_Unified_Runtime_Architecture";
export const REPO_URL = `https://github.com/${OWNER}/${REPO_NAME}`;
export const MODULE_PATH = `github.com/${OWNER}/${REPO_NAME}`;

export const RELEASE_URL = `${REPO_URL}/releases/tag/v${VERSION}`;
export const RELEASES_URL = `${REPO_URL}/releases`;
export const DOWNLOAD_BASE = `${REPO_URL}/releases/download/v${VERSION}`;
export const CHECKSUMS_URL = `${DOWNLOAD_BASE}/SHA256SUMS`;
export const BUILD_INFO_URL = `${DOWNLOAD_BASE}/BUILD-INFO.txt`;

export const README_URL = `${REPO_URL}#readme`;
export const readmeSection = (anchor: string) => `${REPO_URL}/blob/main/README.md#${anchor}`;
export const CHANGELOG_URL = `${REPO_URL}/blob/main/CHANGELOG.md`;
export const LICENSE_URL = `${REPO_URL}/blob/main/LICENSE`;
export const SOURCE_URL = `${REPO_URL}/tree/main`;
export const ISSUES_URL = `${REPO_URL}/issues`;
export const NEW_ISSUE_URL = `${REPO_URL}/issues/new`;
export const CI_URL = `${REPO_URL}/actions/workflows/ci.yml`;

export const GO_INSTALL = `go install ${MODULE_PATH}/cmd/tread@v${VERSION}`;

// Building from source works before and after a release.
export const BUILD_FROM_SOURCE = [
  `git clone ${REPO_URL}.git`,
  `cd ${REPO_NAME}`,
  "go build -o ./bin/tread ./cmd/tread",
  "./bin/tread run examples/hello.tr",
].join("\n");

export const asset = (name: string) => `${DOWNLOAD_BASE}/${name}`;

export interface PlatformBuild {
  label: string; // e.g. "Apple Silicon"
  arch: string; // e.g. "ARM64"
  file: string; // exact release asset name
}

export interface Platform {
  id: "macos" | "linux" | "windows";
  name: string;
  requirement: string;
  builds: PlatformBuild[];
  verify: string; // checksum command
}

export const PLATFORMS: Platform[] = [
  {
    id: "macos",
    name: "macOS",
    requirement: "macOS 12 or later",
    builds: [
      { label: "Apple Silicon", arch: "ARM64", file: `tread_${VERSION}_darwin_arm64.tar.gz` },
      { label: "Intel", arch: "AMD64", file: `tread_${VERSION}_darwin_amd64.tar.gz` },
    ],
    verify: "shasum -a 256 -c SHA256SUMS --ignore-missing",
  },
  {
    id: "linux",
    name: "Linux",
    requirement: "Kernel 3.2 or later · static binary",
    builds: [
      { label: "x86-64", arch: "AMD64", file: `tread_${VERSION}_linux_amd64.tar.gz` },
      { label: "ARM64", arch: "ARM64", file: `tread_${VERSION}_linux_arm64.tar.gz` },
    ],
    verify: "sha256sum -c SHA256SUMS --ignore-missing",
  },
  {
    id: "windows",
    name: "Windows",
    requirement: "Windows 10 / Server 2016 or later",
    builds: [{ label: "x86-64", arch: "AMD64", file: `tread_${VERSION}_windows_amd64.zip` }],
    verify: "Get-FileHash .\\tread_" + VERSION + "_windows_amd64.zip -Algorithm SHA256",
  },
];

export const NAV_LINKS = [
  { id: "overview", label: "Overview" },
  { id: "features", label: "Features" },
  { id: "architecture", label: "Architecture" },
  { id: "examples", label: "Examples" },
  { id: "documentation", label: "Documentation" },
] as const;
