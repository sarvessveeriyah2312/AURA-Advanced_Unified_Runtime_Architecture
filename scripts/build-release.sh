#!/usr/bin/env sh
# Reproducible AURA release build (ADR-0007, packaging: ADR-0008).
#
# Builds the `aura` executable for every supported target and packages
# each one into an archive with LICENSE, README.md, CHANGELOG.md and
# THIRD_PARTY_NOTICES.txt, then writes SHA256SUMS and BUILD-INFO.txt.
# Nothing is tagged, uploaded or published.
#
# Usage:
#   scripts/build-release.sh
#
# Environment:
#   DIST_DIR      output directory (default: dist); it is recreated.
#   AURA_VERSION  version to embed (default: from internal/cli/version.go).
#
# Output (DIST_DIR):
#   aura_<version>_<os>_<arch>.tar.gz   (macOS, Linux)
#   aura_<version>_windows_amd64.zip    (Windows)
#   SHA256SUMS, BUILD-INFO.txt
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
for f in LICENSE README.md CHANGELOG.md; do
	if [ ! -f "$f" ]; then
		echo "build-release: $f is missing; every archive must include it" >&2
		exit 1
	fi
done

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
WORK="${DIST_DIR}/.work"
mkdir -p "$WORK"

echo "==> AURA ${VERSION} ($(go version))"

# The archive writer runs on the build machine (ADR-0008).
go build -o "${WORK}/package" ./scripts/package

# The binaries statically link the Go runtime and standard library,
# whose BSD-3-Clause license requires its notice in binary distributions.
{
	echo "AURA release binaries include the Go runtime and standard library,"
	echo "built with $(go env GOVERSION). They are distributed under this license:"
	echo
	cat "$(go env GOROOT)/LICENSE"
} > "${WORK}/THIRD_PARTY_NOTICES.txt"

for target in $TARGETS; do
	goos=${target%/*}
	goarch=${target#*/}
	name="aura_${VERSION}_${goos}_${goarch}"
	exe=aura
	archive="${name}.tar.gz"
	if [ "$goos" = "windows" ]; then
		exe=aura.exe
		archive="${name}.zip"
	fi
	echo "==> ${goos}/${goarch} -> ${DIST_DIR}/${archive}"
	mkdir -p "${WORK}/${name}"
	CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch \
		go build $FLAGS -ldflags "$LDFLAGS" -o "${WORK}/${name}/${exe}" ./cmd/aura
	"${WORK}/package" -o "${DIST_DIR}/${archive}" -dir "$name" \
		"755:${exe}=${WORK}/${name}/${exe}" \
		"644:LICENSE=LICENSE" \
		"644:README.md=README.md" \
		"644:CHANGELOG.md=CHANGELOG.md" \
		"644:THIRD_PARTY_NOTICES.txt=${WORK}/THIRD_PARTY_NOTICES.txt"
done

rm -rf "$WORK"

(
	cd "$DIST_DIR"
	# Sorted, stable list of artifacts; LC_ALL=C makes the order locale-independent.
	files=$(LC_ALL=C ls aura_* | LC_ALL=C sort)
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
archives:  aura(.exe), LICENSE, README.md, CHANGELOG.md, THIRD_PARTY_NOTICES.txt
EOF

echo "==> checksums (${DIST_DIR}/SHA256SUMS)"
cat "${DIST_DIR}/SHA256SUMS"
echo "OK"
