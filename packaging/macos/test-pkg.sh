#!/usr/bin/env bash
# Tests the macOS .pkg end to end (ADR-0011 §7): install, PATH for new
# login shells, tread doctor, upgrade, and uninstall. Needs sudo; CI only.
#
# Usage:
#   packaging/macos/test-pkg.sh DIR UPGRADE_DIR PLATFORM
set -euo pipefail

dir=${1:?usage: test-pkg.sh DIR UPGRADE_DIR PLATFORM}
updir=${2:?}
platform=${3:?}
pkg=$(ls "$dir"/tread-sdk-*-"$platform".pkg | head -1)
up=$(ls "$updir"/tread-sdk-*-"$platform".pkg | head -1)
ver() { basename "$1" | sed "s/^tread-sdk-//; s/-${platform}\.pkg\$//"; }
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
# A new login shell, as a new Terminal window would start.
login() { env -i HOME="$HOME" USER="$USER" /bin/zsh -l -c "$1"; }

check "install $v1" sudo installer -pkg "$pkg" -target /
check "binary installed" test -x /Library/Tread/bin/tread
check "paths.d entry" grep -qx /Library/Tread/bin /etc/paths.d/tread
check "tread on PATH in a login shell" sh -c "test \"\$(env -i HOME=\"$HOME\" /bin/zsh -l -c 'command -v tread')\" = /Library/Tread/bin/tread"
check "version" login "test \"\$(tread --version)\" = 'tread $v1'"
check "doctor healthy" login "tread doctor | tee /dev/stderr | grep -q '^Installation is healthy'"
check "doctor PATH ok" login "tread doctor | grep -q '^\[OK\]   PATH: tread resolves to this executable'"
check "receipt" sh -c "pkgutil --pkg-info dev.tread.sdk | grep -qx 'version: $v1'"
work=$(mktemp -d)
check "new, build, run" login "cd '$work' && tread new Demo && tread build Demo/src/main.tr && tread run Demo/src/main.tbc | grep -qx 'Hello from Demo!'"

check "upgrade to $v2" sudo installer -pkg "$up" -target /
check "upgraded version" login "test \"\$(tread --version)\" = 'tread $v2'"
check "doctor healthy after upgrade" login "tread doctor | grep -q '^Installation is healthy'"

check "uninstall" sudo /Library/Tread/uninstall.sh
check "files removed" test ! -e /Library/Tread
check "paths.d entry removed" test ! -e /etc/paths.d/tread
check "receipt removed" fails pkgutil --pkg-info dev.tread.sdk
check "tread gone from login shells" login "! command -v tread"
check "project kept" test -f "$work/Demo/src/main.tr"

if [ "$failures" -ne 0 ]; then
	echo "test-pkg: $failures check(s) failed" >&2
	exit 1
fi
echo "OK (.pkg, $platform)"
