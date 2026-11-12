#!/usr/bin/env sh
# Reproducible Tread release build (ADR-0007, ADR-0011).
#
# Builds both editions of `tread` for every supported platform and
# packages each one with the installation layout of ADR-0011 §4:
#
#   tread-<edition>-<version>-<platform>/
#     bin/tread[.exe]
#     docs/LICENSE, README.md, CHANGELOG.md, THIRD_PARTY_NOTICES.txt
#     release
#
# It also writes the Linux SDK .deb packages, copies the install
# scripts, and writes SHA256SUMS, release-manifest.json and
# BUILD-INFO.txt. Every output is byte-reproducible. The macOS .pkg,
# Windows .msi and Linux .rpm need native tools and are built from these
# archives by scripts under packaging/ (see the release workflow).
# Nothing is tagged, uploaded or published.
#
# Usage:
#   scripts/build-release.sh
#
# Environment:
#   DIST_DIR       output directory (default: dist); it is recreated.
#   TREAD_VERSION  version to embed (default: from internal/cli/version.go).
set -eu

cd "$(dirname "$0")/.."

MODULE=$(sed -n 's/^module //p' go.mod)
DEFAULT_VERSION=$(sed -n 's/^var Version = "\(.*\)"$/\1/p' internal/cli/version.go)
VERSION=${TREAD_VERSION:-$DEFAULT_VERSION}
DIST_DIR=${DIST_DIR:-dist}
TARGETS="darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64"
BYTECODE_FORMAT=$(sed -n 's/^	FormatVersion = \([0-9][0-9]*\)$/\1/p' internal/bytecode/encoding.go)

if [ -z "$MODULE" ] || [ -z "$VERSION" ] || [ -z "$BYTECODE_FORMAT" ]; then
	echo "build-release: could not read the module path, version or bytecode format" >&2
	exit 1
fi
# SemVer: MAJOR.MINOR.PATCH with an optional pre-release suffix.
if ! printf '%s\n' "$VERSION" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'; then
	echo "build-release: version '$VERSION' is not SemVer" >&2
	exit 1
fi
for f in LICENSE README.md CHANGELOG.md scripts/install/install.sh scripts/install/install.ps1; do
	if [ ! -f "$f" ]; then
		echo "build-release: $f is missing; every release must include it" >&2
		exit 1
	fi
done

LDFLAGS="-s -w -buildid= -X ${MODULE}/internal/cli.Version=${VERSION}"
FLAGS="-trimpath -buildvcs=false"

rm -rf "$DIST_DIR"
WORK="${DIST_DIR}/.work"
mkdir -p "$WORK"

echo "==> Tread ${VERSION} ($(go version))"

# The release tools run on the build machine.
go build -o "${WORK}/package" ./scripts/package
go build -o "${WORK}/manifest" ./scripts/manifest

# The binaries statically link the Go runtime and standard library,
# whose BSD-3-Clause license requires its notice in binary distributions.
{
	echo "Tread release binaries include the Go runtime and standard library,"
	echo "built with $(go env GOVERSION). They are distributed under this license:"
	echo
	cat "$(go env GOROOT)/LICENSE"
} > "${WORK}/THIRD_PARTY_NOTICES.txt"

# platform GOOS GOARCH prints the release platform name (ADR-0011 §6).
platform() {
	os=$1
	arch=$2
	[ "$os" = darwin ] && os=macos
	[ "$arch" = amd64 ] && arch=x64
	echo "${os}-${arch}"
}

# The Debian version: a SemVer pre-release "-rc.1" becomes "~rc.1" so
# that it sorts before the final release.
DEB_VERSION=$(printf '%s' "$VERSION" | sed 's/-/~/')

for target in $TARGETS; do
	goos=${target%/*}
	goarch=${target#*/}
	plat=$(platform "$goos" "$goarch")
	exe=tread
	ext=tar.gz
	if [ "$goos" = "windows" ]; then
		exe=tread.exe
		ext=zip
	fi
	for edition in sdk runtime; do
		name="tread-${edition}-${VERSION}-${plat}"
		pkg=./cmd/tread
		[ "$edition" = runtime ] && pkg=./cmd/tread-runtime
		echo "==> ${edition} ${goos}/${goarch} -> ${DIST_DIR}/${name}.${ext}"
		mkdir -p "${WORK}/${name}"
		CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch \
			go build $FLAGS -ldflags "$LDFLAGS" -o "${WORK}/${name}/${exe}" "$pkg"
		cat > "${WORK}/${name}/release" <<-EOF
			TREAD_VERSION="${VERSION}"
			TREAD_EDITION="${edition}"
			TREAD_BYTECODE_FORMAT="${BYTECODE_FORMAT}"
			TREAD_PLATFORM="${plat}"
		EOF
		set -- \
			"755:bin/${exe}=${WORK}/${name}/${exe}" \
			"644:docs/LICENSE=LICENSE" \
			"644:docs/README.md=README.md" \
			"644:docs/CHANGELOG.md=CHANGELOG.md" \
			"644:docs/THIRD_PARTY_NOTICES.txt=${WORK}/THIRD_PARTY_NOTICES.txt" \
			"644:release=${WORK}/${name}/release"
		"${WORK}/package" -o "${DIST_DIR}/${name}.${ext}" -dir "$name" "$@"

		if [ "$goos" = linux ] && [ "$edition" = sdk ]; then
			echo "==> sdk ${goos}/${goarch} -> ${DIST_DIR}/${name}.deb"
			cat > "${WORK}/${name}/control" <<-EOF
				Package: tread
				Version: ${DEB_VERSION}
				Architecture: ${goarch}
				Maintainer: Tread project <https://github.com/sarvessveeriyah2312/Tread>
				Section: devel
				Priority: optional
				Homepage: https://treadlang.vercel.app/
				Description: Tread programming language SDK
				 Tread is a language for learning how languages work. This package
				 installs the Tread SDK (compiler, bytecode tools and runtime) in
				 /opt/tread and links /usr/bin/tread to it.
			EOF
			"${WORK}/package" -o "${DIST_DIR}/${name}.deb" -dir opt/tread \
				-control "${WORK}/${name}/control" \
				-link usr/bin/tread=/opt/tread/bin/tread "$@"
		fi
	done
done

cp scripts/install/install.sh scripts/install/install.ps1 "$DIST_DIR/"
chmod 755 "$DIST_DIR/install.sh"
chmod 644 "$DIST_DIR/install.ps1"

"${WORK}/manifest" -version "$VERSION" "$DIST_DIR"
rm -rf "$WORK"

cat > "${DIST_DIR}/BUILD-INFO.txt" <<EOF
version:   ${VERSION}
bytecode:  format ${BYTECODE_FORMAT}
module:    ${MODULE}
toolchain: $(go env GOVERSION)
env:       CGO_ENABLED=0
flags:     ${FLAGS}
ldflags:   ${LDFLAGS}
targets:   ${TARGETS}
editions:  sdk (cmd/tread), runtime (cmd/tread-runtime)
archives:  bin/tread(.exe), docs/{LICENSE,README.md,CHANGELOG.md,THIRD_PARTY_NOTICES.txt}, release
EOF

echo "==> checksums (${DIST_DIR}/SHA256SUMS)"
cat "${DIST_DIR}/SHA256SUMS"
echo "OK"
