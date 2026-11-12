#!/bin/sh
# Tread installer for macOS and Linux (ADR-0011 §7).
#
# Installs the Tread SDK (or the Tread Runtime) for the current user. No
# administrator rights are needed, and Go is not required.
#
#   curl -fsSL https://github.com/sarvessveeriyah2312/Tread/releases/latest/download/install.sh | sh
#   sh install.sh [options]
#
# Options:
#   --runtime            install the Tread Runtime instead of the SDK
#   --version VERSION    install this version (default: the latest release)
#   --prefix DIR         install directory (default: $HOME/.tread)
#   --archive FILE       install from a downloaded archive instead of the network
#   --checksums FILE     SHA256SUMS to verify --archive with (default: next to it)
#   --no-modify-path     do not add Tread to PATH in shell startup files
#   --uninstall          remove an installation made by this script
#   -h, --help           print this help
#
# Safety: the archive is verified against SHA256SUMS before anything is
# installed. Only files this script installed are replaced or removed;
# the install directory must be empty or a previous Tread installation.
# Shell startup files get one marked block, replaced on reinstall and
# removed on uninstall.
set -eu

REPO="https://github.com/sarvessveeriyah2312/Tread"
MANIFEST=".tread-install-manifest"
BEGIN_MARK="# >>> tread >>>"
END_MARK="# <<< tread <<<"

edition=sdk
version=""
prefix="${HOME}/.tread"
archive=""
checksums=""
modify_path=1
action=install

say() { printf '%s\n' "$*"; }
die() {
	printf 'tread install: error: %s\n' "$*" >&2
	exit 1
}

usage() {
	cat <<'USAGE'
Usage: install.sh [options]

Installs the Tread SDK (or Runtime) for the current user.

Options:
  --runtime            install the Tread Runtime instead of the SDK
  --version VERSION    install this version (default: the latest release)
  --prefix DIR         install directory (default: $HOME/.tread)
  --archive FILE       install from a downloaded archive instead of the network
  --checksums FILE     SHA256SUMS to verify --archive with (default: next to it)
  --no-modify-path     do not add Tread to PATH in shell startup files
  --uninstall          remove an installation made by this script
  -h, --help           print this help
USAGE
}

while [ $# -gt 0 ]; do
	case $1 in
	--runtime) edition=runtime ;;
	--sdk) edition=sdk ;;
	--version)
		[ $# -ge 2 ] || die "--version needs a value"
		version=${2#v}
		shift
		;;
	--prefix)
		[ $# -ge 2 ] || die "--prefix needs a value"
		prefix=$2
		shift
		;;
	--archive)
		[ $# -ge 2 ] || die "--archive needs a value"
		archive=$2
		shift
		;;
	--checksums)
		[ $# -ge 2 ] || die "--checksums needs a value"
		checksums=$2
		shift
		;;
	--no-modify-path) modify_path=0 ;;
	--uninstall) action=uninstall ;;
	-h | --help)
		usage
		exit 0
		;;
	*) die "unknown option '$1' (see --help)" ;;
	esac
	shift
done

case $prefix in
/*) ;;
*) prefix="$(pwd)/${prefix}" ;;
esac
prefix=${prefix%/}
[ -n "$prefix" ] || die "refusing to use / as the install directory"

# profiles lists the shell startup files that get the PATH block.
profiles() {
	for f in "$HOME/.profile" "$HOME/.bashrc" "$HOME/.bash_profile" "$HOME/.zshrc" "$HOME/.zprofile"; do
		case $f in
		"$HOME/.profile") printf '%s\n' "$f" ;; # always: read by sh and login shells
		*) [ -f "$f" ] && printf '%s\n' "$f" ;;
		esac
	done
	# zsh does not read .profile; make sure zsh users get the block.
	case ${SHELL:-} in
	*/zsh) [ -f "$HOME/.zshrc" ] || printf '%s\n' "$HOME/.zshrc" ;;
	esac
	return 0
}

# remove_block FILE deletes the marked block, keeping everything else.
remove_block() {
	[ -f "$1" ] || return 0
	grep -qF "$BEGIN_MARK" "$1" || return 0
	tmp="$1.tread-tmp.$$"
	awk -v b="$BEGIN_MARK" -v e="$END_MARK" '
		$0 == b { skip = 1; next }
		$0 == e { skip = 0; next }
		!skip { print }
	' "$1" > "$tmp"
	cat "$tmp" > "$1" # keep the file's owner and permissions
	rm -f "$tmp"
}

add_block() {
	remove_block "$1"
	{
		printf '%s\n' "$BEGIN_MARK"
		printf 'export TREAD_HOME="%s"\n' "$prefix"
		# shellcheck disable=SC2016
		printf '%s\n' 'case ":${PATH}:" in *":${TREAD_HOME}/bin:"*) ;; *) export PATH="${TREAD_HOME}/bin:${PATH}" ;; esac'
		printf '%s\n' "$END_MARK"
	} >> "$1"
}

# remove_installed deletes exactly the files listed in the manifest, then
# any directories that became empty.
remove_installed() {
	[ -f "$prefix/$MANIFEST" ] || return 0
	while IFS= read -r rel; do
		case $rel in
		"" | /* | *..*) continue ;; # never follow suspicious entries
		esac
		rm -f "$prefix/$rel"
	done < "$prefix/$MANIFEST"
	rm -f "$prefix/$MANIFEST"
	for d in bin docs; do
		rmdir "$prefix/$d" 2>/dev/null || true
	done
	rmdir "$prefix" 2>/dev/null || true
}

if [ "$action" = uninstall ]; then
	if [ ! -f "$prefix/$MANIFEST" ]; then
		die "no installation made by this script in $prefix"
	fi
	remove_installed
	profiles | while IFS= read -r f; do remove_block "$f"; done
	if [ -d "$prefix" ]; then
		say "Removed Tread from $prefix (other files there were kept)."
	else
		say "Removed Tread from $prefix."
	fi
	say "Open a new terminal for the PATH change to take effect."
	exit 0
fi

# Platform detection and validation.
case $(uname -s) in
Darwin) os=macos ;;
Linux) os=linux ;;
*) die "unsupported operating system $(uname -s); on Windows use install.ps1" ;;
esac
case $(uname -m) in
x86_64 | amd64) arch=x64 ;;
arm64 | aarch64) arch=arm64 ;;
*) die "unsupported CPU architecture $(uname -m)" ;;
esac
# A shell running under Rosetta reports x86_64 on Apple silicon.
if [ "$os" = macos ] && [ "$arch" = x64 ] && [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = 1 ]; then
	arch=arm64
fi
if [ "$os" = linux ] && [ "$arch" = x64 ] && grep -qi microsoft /proc/version 2>/dev/null && [ ! -d /run/WSL ]; then
	say "warning: WSL 1 is not supported by Go programs; use WSL 2." >&2
fi
platform="${os}-${arch}"

if command -v sha256sum > /dev/null 2>&1; then
	sha256() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum > /dev/null 2>&1; then
	sha256() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
	die "need sha256sum or shasum to verify the download"
fi

tmpdir=$(mktemp -d 2> /dev/null || mktemp -d -t tread)
trap 'rm -rf "$tmpdir"' EXIT INT TERM

download() { # URL FILE
	if command -v curl > /dev/null 2>&1; then
		curl -fsSL --retry 3 -o "$2" "$1"
	elif command -v wget > /dev/null 2>&1; then
		wget -q -O "$2" "$1"
	else
		die "need curl or wget to download Tread"
	fi
}

if [ -z "$archive" ]; then
	if [ -z "$version" ]; then
		# The latest release's tag, from the redirect of /releases/latest.
		if command -v curl > /dev/null 2>&1; then
			url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$REPO/releases/latest") || die "cannot reach $REPO"
		else
			url=$(wget -q --max-redirect=5 -S -O /dev/null "$REPO/releases/latest" 2>&1 | sed -n 's/^ *Location: //p' | tail -1)
		fi
		version=${url##*/v}
		case $version in
		[0-9]*.[0-9]*.[0-9]*) ;;
		*) die "cannot determine the latest Tread version (got '$url'); use --version" ;;
		esac
	fi
	name="tread-${edition}-${version}-${platform}.tar.gz"
	say "Downloading Tread ${version} (${edition}, ${platform})..."
	download "$REPO/releases/download/v${version}/${name}" "$tmpdir/$name" || die "cannot download $name; check that Tread $version exists for $platform"
	download "$REPO/releases/download/v${version}/SHA256SUMS" "$tmpdir/SHA256SUMS" || die "cannot download SHA256SUMS"
	archive="$tmpdir/$name"
	checksums="$tmpdir/SHA256SUMS"
else
	[ -f "$archive" ] || die "archive $archive does not exist"
	[ -n "$checksums" ] || checksums="$(dirname "$archive")/SHA256SUMS"
	[ -f "$checksums" ] || die "no checksum file $checksums; pass --checksums"
fi

# Verify before extracting anything.
base=$(basename "$archive")
want=$(awk -v f="$base" '$2 == f || $2 == "*" f { print $1 }' "$checksums" | head -1)
[ -n "$want" ] || die "$base is not listed in $(basename "$checksums")"
got=$(sha256 "$archive")
[ "$got" = "$want" ] || die "checksum mismatch for $base: expected $want, got $got"
say "Verified ${base} (SHA-256 ${got})."

case $base in
tread-sdk-*) edition=sdk ;;
tread-runtime-*) edition=runtime ;;
esac
case $base in
*-"$platform".tar.gz) ;;
*) die "$base is not for this computer ($platform)" ;;
esac

mkdir "$tmpdir/x"
tar -xzf "$archive" -C "$tmpdir/x" || die "cannot extract $base"
top="$tmpdir/x/${base%.tar.gz}"
[ -x "$top/bin/tread" ] && [ -f "$top/release" ] || die "$base does not contain a Tread installation"
grep -qx "TREAD_PLATFORM=\"$platform\"" "$top/release" || die "$base is not for this computer ($platform)"

# The target must be empty, absent, or a previous installation.
if [ -e "$prefix" ] && [ ! -f "$prefix/$MANIFEST" ]; then
	if [ -n "$(ls -A "$prefix" 2> /dev/null)" ]; then
		die "$prefix exists and is not a Tread installation made by this script; choose another --prefix"
	fi
fi
previous=""
if [ -f "$prefix/release" ]; then
	previous=$(sed -n 's/^TREAD_VERSION="\(.*\)"$/\1/p' "$prefix/release")
fi
remove_installed

mkdir -p "$prefix/bin" "$prefix/docs"
(cd "$top" && find . -type f | sed 's|^\./||' | LC_ALL=C sort) > "$tmpdir/files"
while IFS= read -r rel; do
	mkdir -p "$prefix/$(dirname "$rel")"
	cp "$top/$rel" "$prefix/$rel"
done < "$tmpdir/files"
chmod 755 "$prefix/bin/tread"
cp "$tmpdir/files" "$prefix/$MANIFEST"

"$prefix/bin/tread" --version > /dev/null 2>&1 || die "the installed tread does not run; please report this"

if [ "$modify_path" = 1 ]; then
	profiles | while IFS= read -r f; do add_block "$f"; done
fi

installed=$(sed -n 's/^TREAD_VERSION="\(.*\)"$/\1/p' "$prefix/release")
label="Tread SDK"
[ "$edition" = runtime ] && label="Tread Runtime"
if [ -n "$previous" ] && [ "$previous" != "$installed" ]; then
	say "Upgraded $label $previous -> $installed in $prefix."
else
	say "Installed $label $installed in $prefix."
fi
if [ "$modify_path" = 1 ]; then
	say "Open a new terminal (or run: . \"\$HOME/.profile\"), then check with: tread doctor"
else
	say "Add $prefix/bin to PATH, then check with: tread doctor"
fi
