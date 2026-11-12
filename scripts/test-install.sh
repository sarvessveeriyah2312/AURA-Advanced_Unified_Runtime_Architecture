#!/usr/bin/env bash
# Tests scripts/install/install.sh end to end against built release
# archives (ADR-0011 §7): fresh install, PATH and TREAD_HOME setup,
# tread doctor, reinstall without duplicate PATH entries, upgrade, the
# runtime edition, refusal of tampered archives and foreign directories,
# and uninstall. It never touches the real home directory: HOME points
# at a temporary directory.
#
# Usage:
#   scripts/test-install.sh DIST_DIR [UPGRADE_DIST_DIR]
#
# DIST_DIR is the output of scripts/build-release.sh. UPGRADE_DIST_DIR,
# if given, holds a build with a different version (for example
# TREAD_VERSION=9.9.9 scripts/build-release.sh) to test upgrades.
set -euo pipefail

cd "$(dirname "$0")/.."
dist=$(cd "${1:?usage: scripts/test-install.sh DIST_DIR [UPGRADE_DIST_DIR]}" && pwd)
upgrade=${2:-}
[ -n "$upgrade" ] && upgrade=$(cd "$upgrade" && pwd)
installer="$PWD/scripts/install/install.sh"

case $(uname -s) in Darwin) os=macos ;; Linux) os=linux ;; *) echo "unsupported OS" >&2; exit 1 ;; esac
case $(uname -m) in x86_64 | amd64) arch=x64 ;; arm64 | aarch64) arch=arm64 ;; esac
if [ "$os" = macos ] && [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = 1 ]; then arch=arm64; fi
platform="${os}-${arch}"
version() { sed -n 's/^version: *//p' "$1/BUILD-INFO.txt"; }
v1=$(version "$dist")
sdk="$dist/tread-sdk-${v1}-${platform}.tar.gz"
rt="$dist/tread-runtime-${v1}-${platform}.tar.gz"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
export HOME="$tmp/home"
mkdir -p "$HOME"
export SHELL=/bin/bash
unset TREAD_HOME
# Start from a PATH without any tread.
PATH=$(printf '%s' "$PATH" | tr ':' '\n' | grep -v -i tread | paste -sd: -)
export PATH
failures=0

check() { # NAME COMMAND...
	local name=$1
	shift
	if "$@" >"$tmp/out" 2>&1; then
		echo "ok    $name"
	else
		echo "FAIL  $name"
		sed 's/^/      /' "$tmp/out"
		failures=$((failures + 1))
	fi
}
fails() { ! "$@"; }
# in_new_shell CMD: run CMD in a login-like shell that reads ~/.profile.
in_new_shell() { env -i HOME="$HOME" PATH="/usr/bin:/bin:/usr/sbin:/sbin" sh -c ". \"\$HOME/.profile\"; $1"; }

prefix="$HOME/.tread"
check "fresh install" sh "$installer" --archive "$sdk"
check "binary installed" test -x "$prefix/bin/tread"
check "release file installed" grep -qx "TREAD_EDITION=\"sdk\"" "$prefix/release"
check "profile has one block" test "$(grep -c '>>> tread >>>' "$HOME/.profile")" = 1
check "bashrc untouched when absent" test ! -e "$HOME/.bashrc"
check "tread on PATH in a new shell" in_new_shell "command -v tread | grep -qx '$prefix/bin/tread'"
check "TREAD_HOME set in a new shell" in_new_shell "test \"\$TREAD_HOME\" = '$prefix'"
check "tread --version" in_new_shell "tread --version | grep -qx 'tread $v1'"
check "doctor is healthy" in_new_shell "tread doctor | tee /dev/stderr | grep -q '^Installation is healthy.\$'"
check "doctor checks everything" in_new_shell "tread doctor | grep -c '^\[OK\]' | grep -qx 8"
check "new, build, run" in_new_shell "cd \"\$HOME\" && tread new Demo && cd Demo && tread build src/main.tr && tread run src/main.tbc | grep -qx 'Hello from Demo!'"

echo "# user content kept" >> "$HOME/.profile"
check "reinstall" sh "$installer" --archive "$sdk"
check "reinstall keeps one block" test "$(grep -c '>>> tread >>>' "$HOME/.profile")" = 1
check "reinstall keeps user lines" grep -qx "# user content kept" "$HOME/.profile"
check "PATH entry not duplicated" in_new_shell ". \"\$HOME/.profile\"; test \$(printf '%s' \"\$PATH\" | tr ':' '\n' | grep -c tread) = 1"

touch "$HOME/.zshrc"
check "existing .zshrc gets the block" sh "$installer" --archive "$sdk"
check ".zshrc has one block" test "$(grep -c '>>> tread >>>' "$HOME/.zshrc")" = 1

# Tampered archive: a copy with one changed byte must be rejected.
mkdir "$tmp/bad"
cp "$sdk" "$dist/SHA256SUMS" "$tmp/bad/"
printf 'x' | dd of="$tmp/bad/$(basename "$sdk")" bs=1 seek=100 conv=notrunc 2>/dev/null
check "tampered archive rejected" fails sh "$installer" --prefix "$tmp/elsewhere" --archive "$tmp/bad/$(basename "$sdk")"
check "nothing installed from it" test ! -e "$tmp/elsewhere"
check "unlisted archive rejected" fails sh "$installer" --prefix "$tmp/elsewhere" --archive "$sdk" --checksums /dev/null

# A directory that is not ours is never used.
mkdir -p "$tmp/foreign" && echo data > "$tmp/foreign/keep.txt"
check "foreign directory refused" fails sh "$installer" --prefix "$tmp/foreign" --archive "$sdk" --no-modify-path
check "foreign file kept" grep -qx data "$tmp/foreign/keep.txt"

# Runtime edition in its own prefix, without touching PATH.
check "runtime install" sh "$installer" --prefix "$tmp/rt" --archive "$rt" --no-modify-path
check "runtime runs .tbc" sh -c "'$tmp/rt/bin/tread' run '$HOME/Demo/src/main.tbc' | grep -qx 'Hello from Demo!'"
check "runtime refuses source" fails "$tmp/rt/bin/tread" run "$HOME/Demo/src/main.tr"
check "runtime doctor (no SDK line)" sh -c "'$tmp/rt/bin/tread' doctor | grep -q 'Installation: .*/rt (Tread Runtime' && ! '$tmp/rt/bin/tread' doctor | grep -q 'SDK compiler'"

if [ -n "$upgrade" ]; then
	v2=$(version "$upgrade")
	echo "mine" > "$prefix/notes.txt"
	check "upgrade $v1 -> $v2" sh -c "sh '$installer' --archive '$upgrade/tread-sdk-${v2}-${platform}.tar.gz' | grep -q 'Upgraded Tread SDK $v1 -> $v2'"
	check "upgraded version runs" in_new_shell "tread --version | grep -qx 'tread $v2'"
	check "doctor healthy after upgrade" in_new_shell "tread doctor | grep -q '^Installation is healthy.\$'"
	check "user file kept on upgrade" grep -qx mine "$prefix/notes.txt"
fi

echo "mine" > "$prefix/notes.txt"
check "uninstall" sh "$installer" --uninstall
check "files removed" test ! -e "$prefix/bin/tread" -a ! -e "$prefix/release"
check "user file kept on uninstall" grep -qx mine "$prefix/notes.txt"
check "profile block removed" fails grep -q 'tread' "$HOME/.profile"
check "zshrc block removed" fails grep -q 'tread' "$HOME/.zshrc"
check "user lines kept" grep -qx "# user content kept" "$HOME/.profile"
check "tread gone from new shells" in_new_shell "! command -v tread"
check "second uninstall fails cleanly" fails sh "$installer" --uninstall

if [ "$failures" -ne 0 ]; then
	echo "test-install: $failures check(s) failed" >&2
	exit 1
fi
echo "OK (install.sh, $platform)"
