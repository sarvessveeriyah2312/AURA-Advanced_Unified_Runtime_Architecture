#!/bin/sh
# Builds the macOS installer package for the Tread SDK (ADR-0011 §7).
# Runs on macOS (pkgbuild and productbuild are part of the OS).
#
# Usage:
#   packaging/macos/build-pkg.sh tread-sdk-<version>-macos-<arch>.tar.gz OUT_DIR
#
# The package installs the SDK archive's contents into /Library/Tread,
# adds /etc/paths.d/tread (so new login shells find /Library/Tread/bin),
# and installs /Library/Tread/uninstall.sh. It refuses to install on a
# Mac of the other architecture. Upgrades replace /Library/Tread; nothing
# outside /Library/Tread and /etc/paths.d/tread is touched.
#
# The package is not signed: there is no Developer ID yet (ADR-0011).
set -eu

archive=${1:?usage: build-pkg.sh SDK_ARCHIVE OUT_DIR}
out=${2:?usage: build-pkg.sh SDK_ARCHIVE OUT_DIR}
base=$(basename "$archive" .tar.gz)
case $base in
tread-sdk-*-macos-arm64) arch=arm64 ;;
tread-sdk-*-macos-x64) arch=x86_64 ;;
*) echo "build-pkg: $archive is not a macOS SDK archive" >&2; exit 1 ;;
esac
version=${base#tread-sdk-}
version=${version%-macos-*}
here=$(cd "$(dirname "$0")" && pwd)

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/root/Library/Tread" "$work/root/private/etc/paths.d" "$work/scripts"
tar -xzf "$archive" -C "$work"
cp -R "$work/$base/." "$work/root/Library/Tread/"
printf '/Library/Tread/bin\n' > "$work/root/private/etc/paths.d/tread"
cp "$here/uninstall.sh" "$work/root/Library/Tread/uninstall.sh"
chmod 755 "$work/root/Library/Tread/uninstall.sh" "$work/root/Library/Tread/bin/tread"
cp "$here/preinstall" "$work/scripts/preinstall"
chmod 755 "$work/scripts/preinstall"

pkgbuild --root "$work/root" --identifier dev.tread.sdk --version "$version" \
	--install-location / --scripts "$work/scripts" --ownership recommended \
	"$work/tread-component.pkg"

sed -e "s/@VERSION@/$version/g" -e "s/@ARCH@/$arch/g" "$here/distribution.xml" > "$work/distribution.xml"
mkdir -p "$out"
productbuild --distribution "$work/distribution.xml" --package-path "$work" \
	"$out/$base.pkg"
echo "build-pkg: wrote $out/$base.pkg"
