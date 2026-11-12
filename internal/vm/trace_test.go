package vm_test

import (
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/bytecode"
	"github.com/sarvessveeriyah2312/Tread/internal/vm"
)

// Stack traces for runtime errors (ADR-0009).

// traceText renders a trace as "function@text-of-span" entries, innermost
// first, so tests can assert both the order and the reported locations.
func traceText(src string, e *vm.Error) string {
	parts := make([]string, len(e.Trace))
	for i, f := range e.Trace {
		parts[i] = f.Function + "@" + src[f.Span.Start:f.Span.End]
	}
	return strings.Join(parts, " | ")
}

func TestTraceOrderAndCallSites(t *testing.T) {
	src := "fn main() -> Unit {\n    print(average(10, 0))\n}\n\n" +
		"fn average(total: Int, count: Int) -> Int {\n    return divide(total, count)\n}\n\n" +
		"fn divide(a: Int, b: Int) -> Int {\n    return a / b\n}\n"
	_, err := runSrc(t, src, vm.Config{})
	if err == nil {
		t.Fatal("expected a runtime error")
	}
	want := "divide@a / b | average@divide(total, count) | main@average(10, 0)"
	if got := traceText(src, err); got != want {
		t.Errorf("trace:\n got %s\nwant %s", got, want)
	}
	if err.Trace[0].Span != err.Span || err.Trace[0].Function != err.Function {
		t.Errorf("Trace[0] = %+v, want the error's own function and span", err.Trace[0])
	}
}

func TestTraceErrorInMainHasOneFrame(t *testing.T) {
	src := mainOf("let z = 0\nprint(1 / z)")
	_, err := runSrc(t, src, vm.Config{})
	if err == nil || len(err.Trace) != 1 || err.Trace[0].Function != "main" || err.Trace[0].Span != err.Span {
		t.Fatalf("got %+v", err)
	}
}

func TestTraceErrorInsideCallArgument(t *testing.T) {
	// The error is in g's argument list, before g is called: g must not
	// appear in the trace.
	src := "fn g(n: Int) -> Int {\n    return n\n}\n\n" +
		"fn f(z: Int) -> Int {\n    return g(1 / z)\n}\n" + mainOf("print(f(0))")
	_, err := runSrc(t, src, vm.Config{})
	if err == nil {
		t.Fatal("expected a runtime error")
	}
	if got, want := traceText(src, err), "f@1 / z | main@f(0)"; got != want {
		t.Errorf("trace:\n got %s\nwant %s", got, want)
	}
}

func TestTraceOverflowInCallee(t *testing.T) {
	src := "fn twice(n: Int) -> Int {\n    return n * 2\n}\n" + mainOf("print(twice(9223372036854775807))")
	_, err := runSrc(t, src, vm.Config{})
	if err == nil || err.Code != vm.CodeIntegerOverflow {
		t.Fatalf("got %+v", err)
	}
	if got, want := traceText(src, err), "twice@n * 2 | main@twice(9223372036854775807)"; got != want {
		t.Errorf("trace:\n got %s\nwant %s", got, want)
	}
}

func TestTraceAtFullCallDepth(t *testing.T) {
	src := "fn deep(n: Int) -> Unit {\n    deep(n + 1)\n}\n" + mainOf("deep(1)")
	_, err := runSrc(t, src, vm.Config{})
	if err == nil || err.Code != vm.CodeCallDepth {
		t.Fatalf("got %+v", err)
	}
	if len(err.Trace) != vm.DefaultMaxCallDepth {
		t.Fatalf("trace has %d frames, want %d (one per active frame)", len(err.Trace), vm.DefaultMaxCallDepth)
	}
	last := err.Trace[len(err.Trace)-1]
	if last.Function != "main" || src[last.Span.Start:last.Span.End] != "deep(1)" {
		t.Errorf("outermost frame = %+v, want main at deep(1)", last)
	}
	for i, f := range err.Trace[:len(err.Trace)-1] {
		if f.Function != "deep" || src[f.Span.Start:f.Span.End] != "deep(n + 1)" {
			t.Fatalf("frame %d = %s@%q, want deep@deep(n + 1)", i, f.Function, src[f.Span.Start:f.Span.End])
		}
	}
}

func TestTraceMutualRecursion(t *testing.T) {
	src := "fn ping(n: Int) -> Int {\n    return pong(n - 1)\n}\n\n" +
		"fn pong(n: Int) -> Int {\n    return ping(10 / n)\n}\n" + mainOf("print(ping(4))")
	// ping(4) -> pong(3) -> ping(3) -> pong(2) -> ping(5) -> pong(4) -> ping(2) -> pong(1)
	// -> ping(10) -> pong(9) -> ping(1) -> pong(0): 10 / 0 in pong.
	_, err := runSrc(t, src, vm.Config{})
	if err == nil || err.Code != vm.CodeDivisionByZero {
		t.Fatalf("got %+v", err)
	}
	if err.Trace[0].Function != "pong" || err.Trace[len(err.Trace)-1].Function != "main" {
		t.Fatalf("trace ends: %+v ... %+v", err.Trace[0], err.Trace[len(err.Trace)-1])
	}
	for i := 1; i < len(err.Trace)-1; i++ {
		want := []string{"ping", "pong"}[(i+1)%2]
		if err.Trace[i].Function != want {
			t.Fatalf("frame %d is %s, want %s (alternating)", i, err.Trace[i].Function, want)
		}
	}
}

func TestTraceCallDepthConfigCountsFrames(t *testing.T) {
	src := "fn r(n: Int) -> Int {\n    return r(n + 1)\n}\n" + mainOf("print(r(0))")
	for _, depth := range []int{1, 2, 5} {
		_, err := runSrc(t, src, vm.Config{MaxCallDepth: depth})
		if err == nil || len(err.Trace) != depth {
			t.Errorf("MaxCallDepth %d: trace %d frames, want %d (%v)", depth, len(err.Trace), depth, err)
		}
	}
}

func TestValidationErrorHasNoTrace(t *testing.T) {
	m := &bytecode.Module{Functions: []bytecode.Function{{Name: "main", Code: []bytecode.Instr{{Op: bytecode.Opcode(250)}}}}}
	var out strings.Builder
	err := vm.Run(m, &out, vm.Config{})
	ve, ok := err.(*vm.Error)
	if !ok || ve.Code != vm.CodeInvalidBytecode || ve.HasSpan || len(ve.Trace) != 0 {
		t.Fatalf("got %#v", err)
	}
}
