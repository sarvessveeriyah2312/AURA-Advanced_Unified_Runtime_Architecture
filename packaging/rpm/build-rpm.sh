#!/bin/sh
# Builds the Tread SDK .rpm from a Linux SDK archive (ADR-0011 §7). Needs
# rpmbuild (Fedora: dnf install rpm-build; Ubuntu: apt-get install rpm).
#
# Usage:
#   packaging/rpm/build-rpm.sh tread-sdk-<version>-linux-<arch>.tar.gz OUT_DIR
set -eu

archive=${1:?usage: build-rpm.sh SDK_ARCHIVE OUT_DIR}
out=${2:?usage: build-rpm.sh SDK_ARCHIVE OUT_DIR}
base=$(basename "$archive" .tar.gz)
case $base in
tread-sdk-*-linux-x64) target=x86_64 ;;
tread-sdk-*-linux-arm64) target=aarch64 ;;
*) echo "build-rpm: $archive is not a Linux SDK archive" >&2; exit 1 ;;
esac
version=${base#tread-sdk-}
version=${version%-linux-*}
# RPM versions cannot contain '-'; '~' sorts a pre-release first.
rpm_version=$(printf '%s' "$version" | sed 's/-/~/')
here=$(cd "$(dirname "$0")" && pwd)

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
tar -xzf "$archive" -C "$work"
rpmbuild -bb --target "$target" \
	--define "_topdir $work/rpm" \
	--define "tread_version $rpm_version" \
	--define "tread_src $work/$base" \
	--define "_build_id_links none" \
	"$here/tread.spec"
mkdir -p "$out"
cp "$work"/rpm/RPMS/"$target"/tread-*.rpm "$out/$base.rpm"
echo "build-rpm: wrote $out/$base.rpm"
