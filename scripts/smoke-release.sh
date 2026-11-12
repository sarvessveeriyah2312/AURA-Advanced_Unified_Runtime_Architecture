#!/usr/bin/env bash
# Smoke-tests one Tread release binary (ADR-0008): every CLI command, the
# documented exit codes, and the diagnostics for compile-time errors,
# runtime errors, and wrong file extensions.
#
# Usage (from the repository root):
#   scripts/smoke-release.sh PATH/TO/tread[.exe] [EXPECTED_VERSION]
#
# EXPECTED_VERSION defaults to the version in internal/cli/version.go.
set -u

cd "$(dirname "$0")/.."

bin=${1:?usage: scripts/smoke-release.sh PATH/TO/tread [EXPECTED_VERSION]}
version=${2:-$(sed -n 's/^var Version = "\(.*\)"$/\1/p' internal/cli/version.go)}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
failures=0

# expect NAME WANT_EXIT WANT_STDOUT STDERR_PATTERN -- COMMAND...
# WANT_STDOUT is compared exactly ("*" skips it); STDERR_PATTERN is an
# extended regex that stderr must match ("" requires empty stderr).
expect() {
	local name=$1 want_exit=$2 want_out=$3 err_pat=$4
	shift 5
	local out err code
	"$@" >"$tmp/stdout" 2>"$tmp/stderr"
	code=$?
	out=$(tr -d '\r' <"$tmp/stdout")
	err=$(tr -d '\r' <"$tmp/stderr")
	local ok=1
	[ "$code" = "$want_exit" ] || ok=0
	if [ "$want_out" != "*" ] && [ "$out" != "$want_out" ]; then ok=0; fi
	if [ -z "$err_pat" ]; then
		[ -z "$err" ] || ok=0
	else
		printf '%s\n' "$err" | grep -Eq "$err_pat" || ok=0
	fi
	if [ "$ok" = 1 ]; then
		echo "ok    $name (exit $code)"
	else
		echo "FAIL  $name: exit $code (want $want_exit)"
		echo "      stdout: $out"
		echo "      stderr: $err"
		failures=$((failures + 1))
	fi
}

cat >"$tmp/type_error.tr" <<'EOF'
fn main() -> Unit {
    let x: Int = "text"
}
EOF
cat >"$tmp/runtime_error.tr" <<'EOF'
fn main() -> Unit {
    let zero = 0
    print("start")
    print(10 / zero)
}
EOF
cp examples/hello.tr "$tmp/hello.nova"

expect "help"                 0 "*"              ""                 -- "$bin" help
expect "version"              0 "tread $version" ""                 -- "$bin" version
expect "check hello"          0 ""               ""                 -- "$bin" check examples/hello.tr
expect "run hello"            0 "Hello, Tread!"  ""                 -- "$bin" run examples/hello.tr
expect "check type error"     1 ""               'error\[TR-T001\]' -- "$bin" check "$tmp/type_error.tr"
expect "run runtime error"    1 "start"          'error\[TR-R001\]' -- "$bin" run "$tmp/runtime_error.tr"
expect "wrong extension"      2 ""               '\.tr'             -- "$bin" run "$tmp/hello.nova"
expect "missing file"         2 ""               'missing\.tr'      -- "$bin" run "$tmp/missing.tr"
expect "no arguments"         2 ""               'Usage: tread'     -- "$bin"
expect "unknown command"      2 ""               'unknown command|Usage' -- "$bin" frobnicate

if [ "$failures" -ne 0 ]; then
	echo "smoke-release: $failures check(s) failed for $bin" >&2
	exit 1
fi
echo "OK ($bin)"
