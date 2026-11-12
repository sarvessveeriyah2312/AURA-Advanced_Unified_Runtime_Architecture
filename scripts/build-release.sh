#!/usr/bin/env sh
# Reproducible AURA release build (ADR-0007, docs/13-mvp-hardening.md).
#
# Builds the `aura` executable for every supported target, then writes
# SHA256SUMS and BUILD-INFO.txt. Nothing is tagged, uploaded or published.
#
# Usage:
#   scripts/build-release.sh
#
# Environment:
#   DIST_DIR      output directory (default: dist); it is recreated.
#   AURA_VERSION  version to embed (default: from internal/cli/version.go).
set -eu

cd "$(dirname "$0")/.."

MODULE=$(sed -n 's/^module //p' go.mod)
DEFAULT_VERSION=$(sed -n 's/^var Version = "\(.*\)"$/\1/p' internal/cli/version.go)
VERSION=${AURA_VERSION:-$DEFAULT_VERSION}
DIST_DIR=${DIST_DIR:-dist}
TARGETS="darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64"

if [ -z "$MODULE" ] || [ -z "$VERSION" ]; then
	echo "build-release: could not read the module path or version" >&2
	exit 1
fi
# SemVer: MAJOR.MINOR.PATCH with an optional pre-release suffix.
if ! printf '%s\n' "$VERSION" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'; then
	echo "build-release: version '$VERSION' is not SemVer" >&2
	exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
	sha256() { sha256sum "$@"; }
elif command -v shasum >/dev/null 2>&1; then
	sha256() { shasum -a 256 "$@"; }
else
	echo "build-release: need sha256sum or shasum" >&2
	exit 1
fi

LDFLAGS="-s -w -buildid= -X ${MODULE}/internal/cli.Version=${VERSION}"
FLAGS="-trimpath -buildvcs=false"

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

echo "==> AURA ${VERSION} ($(go version))"
for target in $TARGETS; do
	goos=${target%/*}
	goarch=${target#*/}
	name="aura-${VERSION}-${goos}-${goarch}"
	if [ "$goos" = "windows" ]; then
		name="${name}.exe"
	fi
	echo "==> ${goos}/${goarch} -> ${DIST_DIR}/${name}"
	CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch \
		go build $FLAGS -ldflags "$LDFLAGS" -o "${DIST_DIR}/${name}" ./cmd/aura
done

(
	cd "$DIST_DIR"
	# Sorted, stable list of artifacts; LC_ALL=C makes the order locale-independent.
	files=$(LC_ALL=C ls aura-* | LC_ALL=C sort)
	# shellcheck disable=SC2086
	sha256 $files > SHA256SUMS
)

cat > "${DIST_DIR}/BUILD-INFO.txt" <<EOF
version:   ${VERSION}
module:    ${MODULE}
toolchain: $(go env GOVERSION)
env:       CGO_ENABLED=0
flags:     ${FLAGS}
ldflags:   ${LDFLAGS}
targets:   ${TARGETS}
EOF

echo "==> checksums (${DIST_DIR}/SHA256SUMS)"
cat "${DIST_DIR}/SHA256SUMS"
echo "OK"
