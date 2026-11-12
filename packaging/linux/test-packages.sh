#!/usr/bin/env bash
# Tests the Linux SDK packages end to end (ADR-0011 §7): install, PATH,
# tread doctor, a project build, upgrade, and removal. Run as root or
# with sudo available; CI runs .deb on Ubuntu and .rpm in Fedora.
#
# Usage:
#   packaging/linux/test-packages.sh deb|rpm DIR UPGRADE_DIR PLATFORM
#
# DIR and UPGRADE_DIR hold tread-sdk-<version>-<PLATFORM>.<kind> files
# of two different versions.
set -euo pipefail

kind=${1:?usage: test-packages.sh deb|rpm DIR UPGRADE_DIR PLATFORM}
dir=${2:?}
updir=${3:?}
platform=${4:?}
SUDO=
[ "$(id -u)" = 0 ] || SUDO=sudo

pick() { ls "$1"/tread-sdk-*-"$platform"."$kind" | head -1; }
pkg=$(pick "$dir")
up=$(pick "$updir")
ver() { basename "$1" | sed "s/^tread-sdk-//; s/-${platform}\.${kind}\$//"; }
v1=$(ver "$pkg")
v2=$(ver "$up")
failures=0
check() {
	local name=$1
	shift
	if out=$("$@" 2>&1); then
		echo "ok    $name"
	else
		echo "FAIL  $name"
		printf '%s\n' "$out" | sed 's/^/      /'
		failures=$((failures + 1))
	fi
}
fails() { ! "$@"; }

install() { if [ "$kind" = deb ]; then $SUDO dpkg -i "$1"; else $SUDO rpm -U "$1"; fi; }
remove() { if [ "$kind" = deb ]; then $SUDO dpkg -r tread; else $SUDO rpm -e tread; fi; }

check "install $v1" install "$pkg"
check "tread is /usr/bin/tread" test "$(command -v tread)" = /usr/bin/tread
check "binary in /opt/tread" test -x /opt/tread/bin/tread
check "version" test "$(tread --version)" = "tread $v1"
check "doctor healthy" sh -c "tread doctor | tee /dev/stderr | grep -q '^Installation is healthy'"
check "doctor sees /opt/tread" sh -c "tread doctor | grep -q '^\[OK\]   Installation: /opt/tread (Tread SDK $v1'"
check "doctor PATH ok" sh -c "tread doctor | grep -q '^\[OK\]   PATH: tread resolves to this executable'"
work=$(mktemp -d)
check "new, build, run" sh -c "cd '$work' && tread new Demo && tread build Demo/src/main.tr && tread run Demo/src/main.tbc | grep -qx 'Hello from Demo!'"

check "upgrade to $v2" install "$up"
check "upgraded version" test "$(tread --version)" = "tread $v2"
check "doctor healthy after upgrade" sh -c "tread doctor | grep -q '^Installation is healthy'"
check "project kept" test -f "$work/Demo/src/main.tbc"

check "remove" remove
check "/usr/bin/tread removed" test ! -e /usr/bin/tread
check "/opt/tread/bin removed" test ! -e /opt/tread/bin/tread
check "release removed" test ! -e /opt/tread/release
check "project still kept" test -f "$work/Demo/src/main.tr"

if [ "$failures" -ne 0 ]; then
	echo "test-packages: $failures check(s) failed" >&2
	exit 1
fi
echo "OK ($kind, $platform)"
