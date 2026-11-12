package diagnostic

import (
	"fmt"

	"github.com/your-org/aura/internal/source"
)

// Severity classifies a diagnostic.
type Severity int

const (
	// Error prevents compilation or execution from succeeding.
	Error Severity = iota
	// Warning is reported but does not fail the build.
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "warning"
	}
	return "error"
}

// Related is a secondary location attached to a diagnostic.
type Related struct {
	Span    source.Span
	Message string
}

// Diagnostic is a structured compiler or runtime message (docs/04 §4.4).
type Diagnostic struct {
	Code     Code
	Severity Severity
	Message  string
	Span     source.Span
	Related  []Related
	Help     string
}

// Errorf creates an error diagnostic with a formatted message.
func Errorf(code Code, span source.Span, format string, args ...any) Diagnostic {
	return Diagnostic{
		Code:     code,
		Severity: Error,
		Message:  fmt.Sprintf(format, args...),
		Span:     span,
	}
}

// WithHelp returns a copy of d with help text.
func (d Diagnostic) WithHelp(help string) Diagnostic {
	d.Help = help
	return d
}

// WithRelated returns a copy of d with an additional related span.
func (d Diagnostic) WithRelated(span source.Span, message string) Diagnostic {
	d.Related = append(append([]Related(nil), d.Related...), Related{Span: span, Message: message})
	return d
}
