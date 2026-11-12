package lexer

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
	"github.com/sarvessveeriyah2312/Tread/internal/token"
)

// Lex converts a source file into tokens. The result always ends with an
// EOF token. Lexical errors are returned as diagnostics; Lex never panics
// on user input and always makes progress (docs/03 §3.3).
func Lex(file *source.File) ([]token.Token, []diagnostic.Diagnostic) {
	l := &lexer{file: file, src: file.Content()}
	if file.HasBOM() {
		l.pos = 3 // ADR-0003 L1: one leading BOM is stripped.
	}
	l.run()
	return l.toks, l.diags
}

// frame is one open string literal on the mode stack.
type frame struct {
	open       int  // offset of the opening quote
	inInterp   bool // currently lexing the expression of a ${...}
	interpOpen int  // offset of the '$' of the current ${
}

type lexer struct {
	file  *source.File
	src   []byte
	pos   int
	toks  []token.Token
	diags []diagnostic.Diagnostic
	// stack holds open string literals. Whenever control is in run(), the
	// top frame (if any) is inside an interpolation.
	stack []frame
}

func (l *lexer) span(start, end int) source.Span { return l.file.Span(start, end) }

func (l *lexer) errorf(code diagnostic.Code, start, end int, format string, args ...any) {
	l.diags = append(l.diags, diagnostic.Errorf(code, l.span(start, end), format, args...))
}

func (l *lexer) emit(kind token.Kind, start, end int) {
	l.emitValue(kind, start, end, "")
}

func (l *lexer) emitValue(kind token.Kind, start, end int, value string) {
	l.toks = append(l.toks, token.Token{
		Kind:   kind,
		Span:   l.span(start, end),
		Lexeme: string(l.src[start:end]),
		Value:  value,
	})
}

func (l *lexer) peekByte(offset int) byte {
	if l.pos+offset < len(l.src) {
		return l.src[l.pos+offset]
	}
	return 0
}

func (l *lexer) run() {
	for {
		if l.pos >= len(l.src) {
			l.abandonInterpolation()
			l.emit(token.EOF, l.pos, l.pos)
			return
		}
		c := l.src[l.pos]
		next := l.peekByte(1)
		switch {
		case c == ' ' || c == '\t':
			l.pos++
		case c == '\n' || c == '\r':
			l.abandonInterpolation()
			l.newline()
		case c == '/' && next == '/':
			l.lineComment()
		case c == '/' && next == '*':
			l.blockComment()
		case isDigit(c) || (c == '.' && isDigit(next)):
			l.number()
		case isIdentStart(c):
			l.identifier()
		case c == '"':
			l.stringLiteral()
		case c == '}' && len(l.stack) > 0:
			l.closeInterpolation()
		case c < utf8.RuneSelf:
			l.operator()
		default:
			l.nonASCII()
		}
	}
}

// newline emits a NEWLINE token for the terminator at l.pos.
func (l *lexer) newline() {
	start := l.pos
	if l.src[l.pos] == '\r' && l.peekByte(1) == '\n' {
		l.pos += 2
	} else {
		l.pos++
	}
	l.emit(token.Newline, start, l.pos)
}

// abandonInterpolation handles a line terminator or EOF reached while an
// interpolation is open. Exactly one diagnostic is reported, for the
// innermost open construct, which here is the interpolation (docs/03
// §3.3). A zero-width INVALID token tells the parser that the template
// ended abnormally.
func (l *lexer) abandonInterpolation() {
	if len(l.stack) == 0 {
		return
	}
	top := l.stack[len(l.stack)-1]
	l.errorf(diagnostic.UnterminatedInterp, top.interpOpen, top.interpOpen+2,
		"unterminated interpolation: missing '}' before the end of the line")
	l.emit(token.Invalid, l.pos, l.pos)
	l.stack = nil
}

func (l *lexer) lineComment() {
	reported := false
	for l.pos < len(l.src) && l.src[l.pos] != '\n' && l.src[l.pos] != '\r' {
		reported = l.skipCommentRune(reported)
	}
}

func (l *lexer) blockComment() {
	start := l.pos
	l.pos += 2
	reported := false
	for {
		if l.pos >= len(l.src) {
			l.errorf(diagnostic.UnterminatedBlockComment, start, start+2,
				"unterminated block comment: missing '*/'")
			return
		}
		c := l.src[l.pos]
		switch {
		case c == '*' && l.peekByte(1) == '/':
			l.pos += 2
			return
		case c == '\n' || c == '\r':
			// ADR-0003 L3: one NEWLINE per terminator inside the comment.
			l.abandonInterpolation()
			l.newline()
		default:
			reported = l.skipCommentRune(reported)
		}
	}
}

// skipCommentRune advances over one code point inside a comment. Invalid
// UTF-8 is reported once per comment (ADR-0002 D2.3).
func (l *lexer) skipCommentRune(reported bool) bool {
	r, size := utf8.DecodeRune(l.src[l.pos:])
	if r == utf8.RuneError && size == 1 && !reported {
		l.errorf(diagnostic.InvalidUTF8, l.pos, l.pos+1, "invalid UTF-8 byte 0x%02X in comment", l.src[l.pos])
		reported = true
	}
	l.pos += size
	return reported
}

// number lexes a numeric literal (ADR-0004 item 1). It consumes the
// maximal run of [0-9A-Za-z_.] and classifies it without converting it.
func (l *lexer) number() {
	start := l.pos
	for l.pos < len(l.src) && isNumberRunByte(l.src[l.pos]) {
		l.pos++
	}
	lexeme := string(l.src[start:l.pos])
	switch classifyNumber(lexeme) {
	case numInt:
		l.emit(token.Int, start, l.pos)
	case numFloat:
		l.emit(token.Float, start, l.pos)
	case numLeadingZero:
		l.errorf(diagnostic.LeadingZero, start, l.pos,
			"leading zeros are not allowed in numeric literal %q", lexeme)
		l.emit(token.Invalid, start, l.pos)
	default:
		l.errorf(diagnostic.MalformedNumber, start, l.pos, "malformed numeric literal %q", lexeme)
		l.emit(token.Invalid, start, l.pos)
	}
}

type numberClass int

const (
	numMalformed numberClass = iota
	numInt
	numFloat
	numLeadingZero
)

func classifyNumber(s string) numberClass {
	intPart, frac, hasDot := strings.Cut(s, ".")
	if !allDigits(intPart) || (hasDot && !allDigits(frac)) {
		return numMalformed
	}
	if len(intPart) > 1 && intPart[0] == '0' {
		return numLeadingZero
	}
	if hasDot {
		return numFloat
	}
	return numInt
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

func (l *lexer) identifier() {
	start := l.pos
	for l.pos < len(l.src) && isIdentContinue(l.src[l.pos]) {
		l.pos++
	}
	l.emit(token.Lookup(string(l.src[start:l.pos])), start, l.pos)
}

// stringLiteral starts a new string at the opening quote.
func (l *lexer) stringLiteral() {
	start := l.pos
	l.pos++
	l.stack = append(l.stack, frame{open: start})
	l.stringText(start, true)
}

// closeInterpolation handles the '}' that ends the current ${...} and
// resumes the enclosing string's text.
func (l *lexer) closeInterpolation() {
	start := l.pos
	l.pos++
	l.stack[len(l.stack)-1].inInterp = false
	l.stringText(start, false)
}

// stringText scans string text from l.pos until the closing quote, the
// next "${", or an unterminated end. segStart is the start of the current
// token (the opening quote, or the '}' that closed an interpolation).
func (l *lexer) stringText(segStart int, first bool) {
	var value strings.Builder
	for {
		if l.pos >= len(l.src) {
			l.unterminatedString(segStart)
			return
		}
		c := l.src[l.pos]
		switch {
		case c == '"':
			l.pos++
			kind := token.String
			if !first {
				kind = token.TemplateTail
			}
			l.emitValue(kind, segStart, l.pos, value.String())
			l.stack = l.stack[:len(l.stack)-1]
			return
		case c == '\n' || c == '\r':
			l.unterminatedString(segStart)
			return
		case c == '\\':
			l.escape(&value)
		case c == '$' && l.peekByte(1) == '{':
			l.openInterpolation(segStart, first, value.String())
			return
		default:
			r, size := utf8.DecodeRune(l.src[l.pos:])
			if r == utf8.RuneError && size == 1 {
				l.errorf(diagnostic.InvalidUTF8, l.pos, l.pos+1, "invalid UTF-8 byte 0x%02X in string", c)
			}
			value.WriteRune(r)
			l.pos += size
		}
	}
}

// escape decodes one escape sequence starting at the backslash.
func (l *lexer) escape(value *strings.Builder) {
	start := l.pos
	if l.pos+1 >= len(l.src) || l.src[l.pos+1] == '\n' || l.src[l.pos+1] == '\r' {
		// The string is unterminated; stringText reports it next.
		l.pos++
		return
	}
	switch n := l.src[l.pos+1]; n {
	case '\\', '"', '$':
		value.WriteByte(n)
	case 'n':
		value.WriteByte('\n')
	case 'r':
		value.WriteByte('\r')
	case 't':
		value.WriteByte('\t')
	default:
		r, size := utf8.DecodeRune(l.src[l.pos+1:])
		l.errorf(diagnostic.InvalidEscape, start, l.pos+1+size,
			"invalid escape sequence %q; valid escapes are \\\\ \\\" \\n \\r \\t \\$",
			string(l.src[start:l.pos+1+size]))
		value.WriteRune(r)
		l.pos += 1 + size
		return
	}
	l.pos += 2
}

// openInterpolation emits the template token ending in "${" and switches
// the top frame into interpolation mode.
func (l *lexer) openInterpolation(segStart int, first bool, value string) {
	dollar := l.pos
	l.pos += 2
	kind := token.TemplateHead
	if !first {
		kind = token.TemplateMiddle
	}
	l.emitValue(kind, segStart, l.pos, value)
	top := &l.stack[len(l.stack)-1]
	top.inInterp = true
	top.interpOpen = dollar

	// Empty interpolation: only spaces or tabs before '}'.
	p := l.pos
	for p < len(l.src) && (l.src[p] == ' ' || l.src[p] == '\t') {
		p++
	}
	if p < len(l.src) && l.src[p] == '}' {
		l.errorf(diagnostic.EmptyInterpolation, dollar, dollar+2, "empty interpolation: '${}' needs an expression")
		l.pos = p
		l.emit(token.Invalid, p, p) // stands in for the missing expression
	}
}

// unterminatedString reports the innermost open string (docs/03 §3.3),
// covers the scanned text with an INVALID token, and discards all modes.
func (l *lexer) unterminatedString(segStart int) {
	top := l.stack[len(l.stack)-1]
	l.errorf(diagnostic.UnterminatedString, top.open, top.open+1,
		"unterminated string literal: missing closing '\"'")
	l.emit(token.Invalid, segStart, l.pos)
	l.stack = nil
}

func (l *lexer) operator() {
	start := l.pos
	c := l.src[l.pos]
	next := l.peekByte(1)
	two := func(k token.Kind) {
		l.pos += 2
		l.emit(k, start, l.pos)
	}
	one := func(k token.Kind) {
		l.pos++
		l.emit(k, start, l.pos)
	}
	switch c {
	case '(':
		one(token.LParen)
	case ')':
		one(token.RParen)
	case '{':
		one(token.LBrace)
	case '}':
		one(token.RBrace)
	case ',':
		one(token.Comma)
	case ':':
		one(token.Colon)
	case '+':
		one(token.Plus)
	case '*':
		one(token.Star)
	case '/':
		one(token.Slash)
	case '%':
		one(token.Percent)
	case '-':
		if next == '>' {
			two(token.Arrow)
		} else {
			one(token.Minus)
		}
	case '=':
		if next == '=' {
			two(token.Eq)
		} else {
			one(token.Assign)
		}
	case '!':
		if next == '=' {
			two(token.Ne)
		} else {
			one(token.Not)
		}
	case '<':
		if next == '=' {
			two(token.Le)
		} else {
			one(token.Lt)
		}
	case '>':
		if next == '=' {
			two(token.Ge)
		} else {
			one(token.Gt)
		}
	case '&':
		if next == '&' {
			two(token.AndAnd)
		} else {
			l.invalidCharacter(1)
		}
	case '|':
		if next == '|' {
			two(token.OrOr)
		} else {
			l.invalidCharacter(1)
		}
	default:
		l.invalidCharacter(1)
	}
}

// invalidCharacter reports and skips size bytes at l.pos (L001).
func (l *lexer) invalidCharacter(size int) {
	start := l.pos
	r, _ := utf8.DecodeRune(l.src[start:])
	l.pos += size
	msg := fmt.Sprintf("invalid character %q (U+%04X)", r, r)
	d := diagnostic.Errorf(diagnostic.InvalidCharacter, l.span(start, l.pos), "%s", msg)
	switch {
	case r == ';':
		d = d.WithHelp("Tread does not use semicolons; end statements with a newline")
	case r == 0xFEFF:
		d = d.WithHelp("a byte order mark is only allowed at the start of the file")
	case r >= utf8.RuneSelf:
		d = d.WithHelp("identifiers must be ASCII; non-ASCII text is allowed only in strings and comments")
	}
	l.diags = append(l.diags, d)
}

func (l *lexer) nonASCII() {
	r, size := utf8.DecodeRune(l.src[l.pos:])
	if r == utf8.RuneError && size == 1 {
		l.errorf(diagnostic.InvalidUTF8, l.pos, l.pos+1, "invalid UTF-8 byte 0x%02X", l.src[l.pos])
		l.pos++
		return
	}
	l.invalidCharacter(size)
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

func isIdentStart(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || c == '_'
}

func isIdentContinue(c byte) bool { return isIdentStart(c) || isDigit(c) }

func isNumberRunByte(c byte) bool { return isIdentContinue(c) || c == '.' }
