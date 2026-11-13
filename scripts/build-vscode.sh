#!/usr/bin/env sh
# Builds the VS Code extension (ADR-0015) as dist/tread-vscode-<version>.vsix.
# The version in editors/vscode/package.json must match
# internal/cli/version.go. Needs Node.js and npm; the extension is not
# published to the Marketplace.
#
# Usage:
#   scripts/build-vscode.sh            build into dist/ (or $DIST_DIR)
set -eu

cd "$(dirname "$0")/.."

version=$(sed -n 's/^var Version = "\(.*\)"$/\1/p' internal/cli/version.go)
ext_version=$(node -p 'require("./editors/vscode/package.json").version')
if [ "$version" != "$ext_version" ]; then
	echo "build-vscode: editors/vscode/package.json has version $ext_version, internal/cli/version.go has $version" >&2
	exit 1
fi

out=${DIST_DIR:-dist}
mkdir -p "$out"
out=$(cd "$out" && pwd)

cd editors/vscode
npm ci --no-audit --no-fund
npx --yes @vscode/vsce@3.2.1 package --out "$out/tread-vscode-$version.vsix"
echo "build-vscode: wrote $out/tread-vscode-$version.vsix"
