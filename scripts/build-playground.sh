#!/usr/bin/env sh
# Builds the website playground (/compile/): the Tread SDK compiled to
# WebAssembly (cmd/tread-playground), plus Go's wasm_exec.js loader.
#
# The outputs are committed under website/public/playground/, because the
# website host does not have Go. The build is reproducible (-trimpath and
# an empty build ID, with the Go version from go.mod), so CI rebuilds it
# with --check and fails if the committed files are out of date.
#
# Usage:
#   scripts/build-playground.sh           rebuild the committed files
#   scripts/build-playground.sh --check   verify they are up to date
set -eu

cd "$(dirname "$0")/.."

OUT=website/public/playground
check=false
case "${1:-}" in
"") ;;
--check) check=true ;;
*)
	echo "usage: scripts/build-playground.sh [--check]" >&2
	exit 2
	;;
esac

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" \
	-o "$tmp/tread.wasm" ./cmd/tread-playground
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$tmp/wasm_exec.js"

if $check; then
	for f in tread.wasm wasm_exec.js; do
		if ! cmp -s "$tmp/$f" "$OUT/$f"; then
			echo "$OUT/$f is out of date: run scripts/build-playground.sh and commit the result" >&2
			exit 1
		fi
	done
	echo "playground: $OUT is up to date"
	exit 0
fi

mkdir -p "$OUT"
cp "$tmp/tread.wasm" "$tmp/wasm_exec.js" "$OUT/"
echo "playground: wrote $OUT/tread.wasm ($(wc -c <"$OUT/tread.wasm" | tr -d ' ') bytes) and $OUT/wasm_exec.js"
