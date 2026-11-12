package diagnostic

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sarvessveeriyah2312/AURA-Advanced_Unified_Runtime_Architecture/internal/source"
)

// TraceFrame is one frame of a runtime stack trace (ADR-0009): a
// function and the location it was executing (for the innermost frame)
// or calling from (for every caller).
type TraceFrame struct {
	Function string
	Span     source.Span
}

// Stack trace limits (ADR-0009).
const (
	// traceMaxLines is the most frame lines printed after merging.
	traceMaxLines = 20
	// traceKeep frame lines are kept at each end when the cap applies.
	traceKeep = 10
	// traceExcerptMax is the longest source excerpt, in code points.
	traceExcerptMax = 60
)

// RenderTrace writes the "stack trace:" block of ADR-0009 for frames,
// which are ordered innermost first and end with main. It writes nothing
// when there are fewer than two frames (an error in main) or no file.
//
//	stack trace:
//	  0: divide   trace.aura:10:12  return a / b
//	  1: average  trace.aura:6:12   return divide(total, count)
//	  2: main     trace.aura:2:11   print(average(10, 0))
//
// Consecutive identical frames (same function and location) merge into
// one line labelled with their index range. After merging, at most 20
// lines are printed: the 10 innermost, a count of the frames not shown,
// and the 10 outermost. The output is deterministic and has no trailing
// spaces.
func RenderTrace(w io.Writer, file *source.File, frames []TraceFrame) error {
	if file == nil || len(frames) < 2 {
		return nil
	}

	type group struct {
		first, last int
		frame       TraceFrame
	}
	var groups []group
	for i, f := range frames {
		if n := len(groups); n > 0 && groups[n-1].frame == f {
			groups[n-1].last = i
			continue
		}
		groups = append(groups, group{first: i, last: i, frame: f})
	}

	shown, gap, omitted := groups, -1, 0
	if len(groups) > traceMaxLines {
		for _, g := range groups[traceKeep : len(groups)-traceKeep] {
			omitted += g.last - g.first + 1
		}
		shown = append(append([]group{}, groups[:traceKeep]...), groups[len(groups)-traceKeep:]...)
		gap = traceKeep
	}

	type row struct{ label, name, loc, text string }
	rows := make([]row, len(shown))
	var labelW, nameW, locW int
	for i, g := range shown {
		label := strconv.Itoa(g.first) + ":"
		if g.last != g.first {
			label = strconv.Itoa(g.first) + "-" + strconv.Itoa(g.last) + ":"
		}
		pos := file.Position(g.frame.Span.Start)
		r := row{
			label: label,
			name:  g.frame.Function,
			loc:   fmt.Sprintf("%s:%d:%d", file.Name(), pos.Line, pos.Column),
			text:  traceExcerpt(file, pos.Line),
		}
		rows[i] = r
		labelW = max(labelW, textWidth(r.label))
		nameW = max(nameW, textWidth(r.name))
		locW = max(locW, textWidth(r.loc))
	}

	var b strings.Builder
	b.WriteString("stack trace:\n")
	for i, r := range rows {
		if i == gap {
			fmt.Fprintf(&b, "  ... %d more frames not shown\n", omitted)
		}
		line := "  " + padRight(r.label, labelW) + " " + padRight(r.name, nameW) + "  " + padRight(r.loc, locW) + "  " + r.text
		b.WriteString(strings.TrimRight(line, " "))
		b.WriteByte('\n')
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// traceExcerpt returns source line lineNo for a trace line: invalid UTF-8
// becomes U+FFFD, tabs become single spaces, surrounding whitespace is
// trimmed, and text longer than traceExcerptMax code points is cut and
// ends with "...".
func traceExcerpt(file *source.File, lineNo int) string {
	if lineNo < 1 || lineNo > file.LineCount() {
		return ""
	}
	start, end := file.LineBounds(lineNo)
	line := file.Content()[start:end]
	var b strings.Builder
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRune(line[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			b.WriteRune(utf8.RuneError)
		case r == '\t':
			b.WriteByte(' ')
		default:
			b.Write(line[i : i+size])
		}
		i += size
	}
	text := strings.TrimSpace(b.String())
	if utf8.RuneCountInString(text) > traceExcerptMax {
		runes := []rune(text)
		text = strings.TrimRight(string(runes[:traceExcerptMax-3]), " ") + "..."
	}
	return text
}

// textWidth is the terminal display width of s (wide characters count 2).
func textWidth(s string) int { return displayWidth([]byte(s)) }

// padRight pads s with spaces to display width w.
func padRight(s string, w int) string {
	if n := w - textWidth(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}
