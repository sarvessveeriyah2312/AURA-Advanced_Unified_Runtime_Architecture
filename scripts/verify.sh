#!/usr/bin/env sh
# Local quality gate for Tread. CI runs this same script.
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

echo "==> tread smoke test"
output=$(go run ./cmd/tread run examples/hello.tr)
if [ "$output" != "Hello, Tread!" ]; then
	echo "tread run examples/hello.tr printed: $output" >&2
	exit 1
fi

echo "OK"
