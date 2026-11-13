package vm_test

import (
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/vm"
)

// Loops (ADR-0012).

func TestV21WhileAndFor(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"while counts", "let mut i = 0\nwhile i < 3 {\nprint(i)\ni = i + 1\n}", "0\n1\n2\n"},
		{"while false never runs", "while false {\nprint(\"no\")\n}\nprint(\"done\")", "done\n"},
		{"for is half-open", "for i in 0..3 {\nprint(i)\n}", "0\n1\n2\n"},
		{"empty and reversed ranges", "for i in 3..3 {\nprint(i)\n}\nfor i in 5..1 {\nprint(i)\n}\nprint(\"done\")", "done\n"},
		{"negative range", "for i in -2..1 {\nprint(i)\n}", "-2\n-1\n0\n"},
		{"break and continue", "for i in 0..10 {\nif i % 2 == 1 {\ncontinue\n}\nif i > 6 {\nbreak\n}\nprint(i)\n}", "0\n2\n4\n6\n"},
		{"while continue re-tests condition", "let mut i = 0\nwhile i < 4 {\ni = i + 1\nif i == 2 {\ncontinue\n}\nprint(i)\n}", "1\n3\n4\n"},
		{"break leaves only the inner loop", "for i in 0..3 {\nfor j in 0..3 {\nif j > i {\nbreak\n}\nprint(\"${i}${j}\")\n}\n}", "00\n10\n11\n20\n21\n22\n"},
		{"bounds evaluated once", "let mut n = 3\nfor i in 0..n {\nn = n + 1\nprint(i)\n}\nprint(n)", "0\n1\n2\n6\n"},
		{"start before end", "for i in say(\"start\", 0)..say(\"end\", 2) {\nprint(i)\n}", "start\nend\n0\n1\n"},
		{"range up to the maximum Int", "for i in 9223372036854775805..9223372036854775807 {\nprint(i)\n}", "9223372036854775805\n9223372036854775806\n"},
		{"range from the minimum Int", "for i in -9223372036854775808..-9223372036854775806 {\nprint(i)\n}", "-9223372036854775808\n-9223372036854775807\n"},
		{"fresh let each iteration", "for i in 0..2 {\nlet x = i * 10\nlet mut y = x\ny = y + 1\nprint(y)\n}", "1\n11\n"},
	}
	say := "fn say(s: String, v: Int) -> Int {\nprint(s)\nreturn v\n}\n"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runSrc(t, say+mainOf(tc.body), vm.Config{})
			if err != nil || out != tc.want {
				t.Errorf("got %q %+v, want %q", out, err, tc.want)
			}
		})
	}
}

// Loops replace deep recursion: a million iterations use one frame.
func TestV22LoopsNeedNoCallDepth(t *testing.T) {
	out, err := runSrc(t, mainOf("let mut total = 0\nfor i in 0..1000000 {\ntotal = total + i\n}\nprint(total)"), vm.Config{})
	if err != nil || out != "499999500000\n" {
		t.Errorf("got %q %+v", out, err)
	}
}

func TestV23ReturnFromLoop(t *testing.T) {
	src := "fn find(target: Int) -> Int {\nlet mut i = 0\nwhile true {\nif i * i >= target {\nreturn i\n}\ni = i + 1\n}\n}\n" +
		"fn firstEven(from: Int) -> Int {\nfor i in from..from + 10 {\nif i % 2 == 0 {\nreturn i\n}\n}\nreturn -1\n}\n" +
		mainOf("print(find(50))\nprint(firstEven(7))")
	out, err := runSrc(t, src, vm.Config{})
	if err != nil || out != "8\n8\n" {
		t.Errorf("got %q %+v", out, err)
	}
}

func TestV24InstructionLimitStopsInfiniteLoop(t *testing.T) {
	src := mainOf("while true {\n}")
	_, err := runSrc(t, src, vm.Config{MaxInstructions: 1000})
	if err == nil || err.Code != vm.CodeInstructionLimit {
		t.Fatalf("got %+v", err)
	}
}

func TestV25RuntimeErrorInsideLoop(t *testing.T) {
	src := mainOf("let mut d = 2\nwhile true {\nprint(10 / d)\nd = d - 1\n}")
	out, err := runSrc(t, src, vm.Config{})
	if err == nil || err.Code != vm.CodeDivisionByZero || spanText(src, err) != "10 / d" || out != "5\n10\n" {
		t.Errorf("got %q %+v (span %q)", out, err, spanText(src, err))
	}
	if len(err.Trace) != 1 || err.Trace[0].Function != "main" {
		t.Errorf("trace = %+v", err.Trace)
	}
}
