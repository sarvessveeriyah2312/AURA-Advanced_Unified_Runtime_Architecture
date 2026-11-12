package diagnostic

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/your-org/aura/internal/source"
)

// Render writes d in the human-readable format of docs/04 §4.4:
//
//	error[AURA-T001]: expected Int but found String
//	  --> examples/main.aura:4:18
//	   |
//	 4 | let total: Int = "five"
//	   |                  ^^^^^^
//
// The reported column counts code points; the caret is placed by display
// width (docs/04 §4.4.1). The span must belong to file. If file is nil
// (a diagnostic without a source location, such as a bytecode validation
// error), only the header line and help text are written.
func Render(w io.Writer, file *source.File, d Diagnostic) error {
	var b strings.Builder
	if file == nil {
		fmt.Fprintf(&b, "%s[%s]: %s\n", d.Severity, d.Code, d.Message)
		if d.Help != "" {
			fmt.Fprintf(&b, "  = help: %s\n", d.Help)
		}
		_, err := io.WriteString(w, b.String())
		return err
	}
	pos := file.Position(d.Span.Start)
	lineNo := strconv.Itoa(pos.Line)
	pad := strings.Repeat(" ", len(lineNo))

	fmt.Fprintf(&b, "%s[%s]: %s\n", d.Severity, d.Code, d.Message)
	fmt.Fprintf(&b, "%s --> %s:%d:%d\n", pad, file.Name(), pos.Line, pos.Column)
	fmt.Fprintf(&b, " %s |\n", pad)

	start, end := file.LineBounds(pos.Line)
	content := file.Content()
	line := content[start:end]
	fmt.Fprintf(&b, " %s | %s\n", lineNo, displayLine(line))

	spanStart := clamp(d.Span.Start, start, end)
	spanEnd := clamp(d.Span.End, spanStart, end)
	prefix := displayWidth(content[start:spanStart])
	carets := displayWidth(content[spanStart:spanEnd])
	if carets < 1 {
		carets = 1
	}
	fmt.Fprintf(&b, " %s | %s%s\n", pad, strings.Repeat(" ", prefix), strings.Repeat("^", carets))

	if d.Help != "" {
		fmt.Fprintf(&b, " %s = help: %s\n", pad, d.Help)
	}
	for _, r := range d.Related {
		rp := file.Position(r.Span.Start)
		fmt.Fprintf(&b, " %s = note: %s (%s:%d:%d)\n", pad, r.Message, file.Name(), rp.Line, rp.Column)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// displayLine prepares a source line for printing: tabs expand to spaces
// and invalid UTF-8 bytes become U+FFFD.
func displayLine(line []byte) string {
	var b strings.Builder
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRune(line[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			b.WriteRune(utf8.RuneError)
		case r == '\t':
			b.WriteString(strings.Repeat(" ", tabWidth))
		default:
			b.Write(line[i : i+size])
		}
		i += size
	}
	return b.String()
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
