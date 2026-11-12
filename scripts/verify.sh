#!/usr/bin/env sh
# Local quality gate for AURA. CI runs this same script.
# Usage: scripts/verify.sh
set -eu

cd "$(dirname "$0")/.."

echo "==> gofmt"
unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
	echo "The following files need gofmt -w:" >&2
	echo "$unformatted" >&2
	exit 1
fi

echo "==> go vet"
go vet ./...

echo "==> go test"
go test ./...

echo "==> go build"
go build ./...

echo "==> aura smoke test"
output=$(go run ./cmd/aura run examples/hello.aura)
if [ "$output" != "Hello, AURA!" ]; then
	echo "aura run examples/hello.aura printed: $output" >&2
	exit 1
fi

echo "OK"
