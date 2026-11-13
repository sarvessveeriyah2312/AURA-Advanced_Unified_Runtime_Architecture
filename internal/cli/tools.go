package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// format implements `tread fmt [--check] <file.tr>...` (ADR-0015). It
// prints nothing when every file is already formatted (or was
// rewritten); with --check it lists the files that are not formatted.
func (t *tool) format(args []string) int {
	check := false
	var files []string
	for _, a := range args {
		switch {
		case a == "--check":
			check = true
		case strings.HasPrefix(a, "-"):
			return t.usageError("unknown flag %q for fmt", a)
		default:
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		return t.usageError("fmt requires at least one file argument")
	}
	code := ExitOK
	errs := &separated{w: t.stderr}
	for _, path := range files {
		file, c := t.readSource(path)
		if file == nil {
			code = max(code, c)
			continue
		}
		out, diags, err := t.compiler.Format(file)
		switch {
		case len(diags) > 0:
			renderAll(errs.next(), source.Set{file}, diags)
			code = max(code, exitCodeForAll(diags))
		case err != nil:
			fmt.Fprintf(errs.next(), "error: %v\n", err)
			code = max(code, ExitFailure)
		case bytes.Equal(out, file.Content()):
		case check:
			fmt.Fprintln(t.stdout, path)
			code = max(code, ExitFailure)
		default:
			if err := writeFileAtomic(path, out); err != nil {
				fmt.Fprintf(t.stderr, "error: cannot write %s: %v\n", path, err)
				code = max(code, ExitFailure)
			}
		}
	}
	return code
}

// test implements `tread test <file.tr>...` (ADR-0015): each test runs on
// its own; its output is shown only when it fails.
func (t *tool) test(args []string) int {
	if len(args) == 0 {
		return t.usageError("test requires at least one file argument")
	}
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return t.usageError("unknown flag %q for test", a)
		}
	}
	code := ExitOK
	errs := &separated{w: t.stderr}
	passed, failed := 0, 0
	for _, path := range args {
		file, c := t.readSource(path)
		if file == nil {
			code = max(code, c)
			continue
		}
		module, files, diags := t.compiler.CompileTests(file)
		if len(diags) > 0 {
			renderAll(errs.next(), files, diags)
			code = max(code, exitCodeForAll(diags))
			continue
		}
		if len(args) > 1 {
			fmt.Fprintf(t.stdout, "%s\n", path)
		}
		if module == nil {
			fmt.Fprintf(t.stdout, "no tests in %s\n", path)
			continue
		}
		for _, test := range module.Tests {
			m := *module
			m.Main = test.Func
			var out bytes.Buffer
			failure, err := execute(&m, &out, t.limits)
			if err != nil {
				fmt.Fprintf(t.stderr, "error: cannot capture test output: %v\n", err)
				return ExitInternal
			}
			if failure == nil {
				passed++
				fmt.Fprintf(t.stdout, "ok    %s\n", test.Name)
				continue
			}
			failed++
			code = max(code, ExitFailure)
			fmt.Fprintf(t.stdout, "FAIL  %s\n", test.Name)
			var report bytes.Buffer
			report.Write(out.Bytes())
			if out.Len() > 0 && !bytes.HasSuffix(out.Bytes(), []byte("\n")) {
				report.WriteByte('\n')
			}
			t.renderFailure(&report, failure, files)
			fmt.Fprint(t.stdout, indent(report.String(), "      "))
		}
	}
	if passed+failed > 0 {
		fmt.Fprintf(t.stdout, "%d %s: %d passed, %d failed\n", passed+failed, plural(passed+failed, "test"), passed, failed)
	}
	return code
}

// renderFailure writes a runtime error and its stack trace (ADR-0009).
func (t *tool) renderFailure(w *bytes.Buffer, failure *runtimeFailure, files source.Set) {
	src := files
	if !failure.withSource {
		src = nil
	}
	renderAll(w, src, []diagnostic.Diagnostic{failure.diag})
	if len(src) > 0 && len(failure.trace) >= 2 {
		w.WriteByte('\n')
		_ = diagnostic.RenderTraceSet(w, src, failure.trace)
	}
}

func indent(s, prefix string) string {
	lines := strings.SplitAfter(s, "\n")
	var b strings.Builder
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			b.WriteString(strings.TrimLeft(l, " \t"))
			continue
		}
		b.WriteString(prefix + l)
	}
	return b.String()
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// lsp implements `tread lsp` (ADR-0015): the language server on stdin and
// stdout. Logs go to stderr.
func (t *tool) lsp() int {
	ls, ok := t.compiler.(LanguageServer)
	if !ok {
		fmt.Fprintln(t.stderr, "error: this build of Tread has no language server")
		return ExitFailure
	}
	if err := ls.ServeLSP(os.Stdin, t.stdout); err != nil {
		fmt.Fprintf(t.stderr, "error: language server: %v\n", err)
		return ExitFailure
	}
	return ExitOK
}

// separated writes a blank line between successive error reports.
type separated struct {
	w    io.Writer
	used bool
}

func (s *separated) next() io.Writer {
	if s.used {
		fmt.Fprintln(s.w)
	}
	s.used = true
	return s.w
}
