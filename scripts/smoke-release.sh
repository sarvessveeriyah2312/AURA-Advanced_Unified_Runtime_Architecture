#!/usr/bin/env bash
# Smoke-tests the Tread release binaries of one platform (ADR-0008,
# ADR-0011): every CLI command of the SDK and of the runtime, the
# documented exit codes, the diagnostics for compile-time errors,
# runtime errors and wrong file extensions, and .tbc files built by the
# SDK and run by the runtime.
#
# Usage (from the repository root):
#   scripts/smoke-release.sh SDK/bin/tread[.exe] RUNTIME/bin/tread[.exe] [EXPECTED_VERSION]
#
# EXPECTED_VERSION defaults to the version in internal/cli/version.go.
set -u

cd "$(dirname "$0")/.."

usage="usage: scripts/smoke-release.sh SDK_TREAD RUNTIME_TREAD [EXPECTED_VERSION]"
bin=${1:?$usage}
rt=${2:?$usage}
version=${3:-$(sed -n 's/^var Version = "\(.*\)"$/\1/p' internal/cli/version.go)}
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

cp examples/hello.tr "$tmp/hello.tr"
cp "$tmp/runtime_error.tr" "$tmp/rterr.tr"
printf 'TRBC' >"$tmp/garbage.tbc"

# SDK edition.
expect "help"                 0 "*"              ""                 -- "$bin" help
expect "version"              0 "tread $version" ""                 -- "$bin" version
expect "sdk --version"        0 "Tread SDK $version" ""             -- "$bin" sdk --version
expect "runtime --version"    0 "Tread Runtime $version (bytecode format 1)" "" -- "$bin" runtime --version
expect "check hello"          0 ""               ""                 -- "$bin" check examples/hello.tr
expect "run hello"            0 "Hello, Tread!"  ""                 -- "$bin" run examples/hello.tr
expect "check type error"     1 ""               'error\[TR-T001\]' -- "$bin" check "$tmp/type_error.tr"
expect "run runtime error"    1 "start"          'error\[TR-R001\]' -- "$bin" run "$tmp/runtime_error.tr"
expect "wrong extension"      2 ""               '\.tr'             -- "$bin" run "$tmp/hello.nova"
expect "missing file"         2 ""               'missing\.tr'      -- "$bin" run "$tmp/missing.tr"
expect "no arguments"         2 ""               'Usage: tread'     -- "$bin"
expect "unknown command"      2 ""               'unknown command|Usage' -- "$bin" frobnicate
expect "build hello"          0 ""               ""                 -- "$bin" build "$tmp/hello.tr"
expect "build runtime error"  0 ""               ""                 -- "$bin" build "$tmp/rterr.tr"
expect "build type error"     1 ""               'error\[TR-T001\]' -- "$bin" build "$tmp/type_error.tr"
expect "run hello.tbc"        0 "Hello, Tread!"  ""                 -- "$bin" run "$tmp/hello.tbc"
expect "invalid .tbc"         1 ""               'error\[TR-R006\]' -- "$bin" run "$tmp/garbage.tbc"
expect "new project"          0 "*"              ""                 -- sh -c "cd '$tmp' && '$bin' new Demo"
expect "run new project"      0 "Hello from Demo!" ""               -- "$bin" run "$tmp/Demo/src/main.tr"
expect "doctor"               0 "*"              ""                 -- "$bin" doctor

# Runtime edition, running what the SDK built.
expect "rt version"           0 "tread $version (runtime)" ""       -- "$rt" version
expect "rt runtime --version" 0 "Tread Runtime $version (bytecode format 1)" "" -- "$rt" runtime --version
expect "rt sdk --version"     1 ""               'SDK is not installed' -- "$rt" sdk --version
expect "rt run hello.tbc"     0 "Hello, Tread!"  ""                 -- "$rt" run "$tmp/hello.tbc"
expect "rt runtime error"     1 "start"          'rterr\.tr:4:' -- "$rt" run "$tmp/rterr.tbc"
expect "rt invalid .tbc"      1 ""               'error\[TR-R006\]' -- "$rt" run "$tmp/garbage.tbc"
expect "rt refuses source"    2 ""               'needs the Tread SDK' -- "$rt" run "$tmp/hello.tr"
expect "rt refuses check"     2 ""               'needs the Tread SDK' -- "$rt" check "$tmp/hello.tr"
expect "rt refuses build"     2 ""               'needs the Tread SDK' -- "$rt" build "$tmp/hello.tr"
expect "rt doctor"            0 "*"              ""                 -- "$rt" doctor

if [ "$failures" -ne 0 ]; then
	echo "smoke-release: $failures check(s) failed for $bin" >&2
	exit 1
fi
echo "OK ($bin, $rt)"
