package lexer

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
	"github.com/sarvessveeriyah2312/Tread/internal/token"
)

const bom = "\xef\xbb\xbf"

// tokCheck asserts the span and/or position of the token at index idx
// (idx -1 means the EOF token).
type tokCheck struct {
	idx  int
	span string // "[s,e)" or ""
	pos  string // "line:col" or ""
}

type lexCase struct {
	id     string
	src    string
	tokens string   // space-separated description, EOF omitted
	diags  []string // "L001@1:8", optionally "L004@1:2[1,3)" to assert the span
	checks []tokCheck
}

// lexCases implements the required lexer test cases L1-L68 of docs/03
// §3.4 (L69 is the fuzz target in fuzz_test.go). Cases with several
// inputs use suffixes a, b, c.
var lexCases = []lexCase{
	// Identifiers, keywords, whitespace, and line structure.
	{id: "L1", src: "let x = 1", tokens: "let IDENT(x) = INT(1)"},
	{id: "L2", src: "let letter mut mutable", tokens: "let IDENT(letter) mut IDENT(mutable)"},
	{id: "L3", src: "_ _x1 X_9 a__b", tokens: "IDENT(_) IDENT(_x1) IDENT(X_9) IDENT(a__b)"},
	{id: "L4", src: "Foo foo FOO", tokens: "IDENT(Foo) IDENT(foo) IDENT(FOO)"},
	{id: "L5", src: "let café = 1", tokens: "let IDENT(caf) = INT(1)", diags: []string{"L001@1:8"}},
	{id: "L6", src: "let 日本 = 1", tokens: "let = INT(1)", diags: []string{"L001@1:5", "L001@1:6"}},
	{id: "L7", src: "class interface pub", tokens: "RESERVED(class) RESERVED(interface) pub"},
	{id: "L8", src: "a\nb", tokens: "IDENT(a) NEWLINE IDENT(b)", checks: []tokCheck{{idx: 2, pos: "2:1"}}},
	{id: "L9", src: "a\r\nb", tokens: "IDENT(a) NEWLINE IDENT(b)", checks: []tokCheck{{idx: 1, span: "[1,3)"}, {idx: 2, pos: "2:1"}}},
	{id: "L10", src: "a\rb", tokens: "IDENT(a) NEWLINE IDENT(b)", checks: []tokCheck{{idx: 1, span: "[1,2)"}, {idx: 2, pos: "2:1"}}},
	{id: "L11", src: "a\r\r\nb", tokens: "IDENT(a) NEWLINE NEWLINE IDENT(b)", checks: []tokCheck{{idx: 3, pos: "3:1"}}},
	{id: "L12", src: "\n\n\n", tokens: "NEWLINE NEWLINE NEWLINE"},
	{id: "L13", src: "a // note\nb", tokens: "IDENT(a) NEWLINE IDENT(b)"},
	{id: "L14", src: "a // note", tokens: "IDENT(a)"},
	{id: "L15", src: "a /* c */ b", tokens: "IDENT(a) IDENT(b)"},
	{id: "L16", src: "a /* x\ny\r\nz\rw */ b", tokens: "IDENT(a) NEWLINE NEWLINE NEWLINE IDENT(b)", checks: []tokCheck{{idx: 4, pos: "4:6"}}},
	{id: "L17", src: "a /* never closed", tokens: "IDENT(a)", diags: []string{"L005@1:3"}},
	{id: "L18", src: "/* /* */ */", tokens: "* /"},
	{id: "L19", src: bom + "let", tokens: "let", checks: []tokCheck{{idx: 0, span: "[3,6)", pos: "1:1"}}},
	{id: "L20", src: bom + bom + "let", tokens: "let", diags: []string{"L001@1:1"}, checks: []tokCheck{{idx: 0, pos: "1:2"}}},
	{id: "L21", src: "a " + bom, tokens: "IDENT(a)", diags: []string{"L001@1:3"}},
	{id: "L22", src: "\tlet", tokens: "let", checks: []tokCheck{{idx: 0, pos: "1:2"}}},
	{id: "L23", src: "Int Float String Bool Unit", tokens: "IDENT(Int) IDENT(Float) IDENT(String) IDENT(Bool) IDENT(Unit)"},

	// Strings, escapes, and interpolation.
	{id: "L24", src: `"a\\b\"c\n\r\t\$"`, tokens: `STRING("a\\b\"c\n\r\t$")`},
	{id: "L25", src: `"\q"`, tokens: `STRING("q")`, diags: []string{"L004@1:2[1,3)"}},
	{id: "L26", src: `"\{" "\x" "\` + "u0041\"", tokens: `STRING("{") STRING("x") STRING("u0041")`, diags: []string{"L004@1:2", "L004@1:7", "L004@1:12"}},
	{id: "L27", src: `"abc`, tokens: "INVALID", diags: []string{"L003@1:1"}},
	{id: "L28", src: "\"abc\nx", tokens: "INVALID NEWLINE IDENT(x)", diags: []string{"L003@1:1"}},
	{id: "L29", src: `"\"`, tokens: "INVALID", diags: []string{"L003@1:1"}},
	{id: "L30", src: `"日本🚀"`, tokens: `STRING("日本🚀")`, checks: []tokCheck{{idx: 0, span: "[0,12)"}}},
	{id: "L31", src: `"a${x}b"`, tokens: `HEAD("a") IDENT(x) TAIL("b")`},
	{id: "L32", src: `"${a}-${b}"`, tokens: `HEAD("") IDENT(a) MIDDLE("-") IDENT(b) TAIL("")`},
	{id: "L33", src: `"${a + 1}"`, tokens: `HEAD("") IDENT(a) + INT(1) TAIL("")`},
	{id: "L34", src: `"${f("x")}"`, tokens: `HEAD("") IDENT(f) ( STRING("x") ) TAIL("")`},
	{id: "L35", src: `"${f("${y}")}"`, tokens: `HEAD("") IDENT(f) ( HEAD("") IDENT(y) TAIL("") ) TAIL("")`},
	{id: "L36", src: `"cost: $5 ${x}"`, tokens: `HEAD("cost: $5 ") IDENT(x) TAIL("")`},
	{id: "L37", src: `"$" "$$" "a$b"`, tokens: `STRING("$") STRING("$$") STRING("a$b")`},
	{id: "L38", src: `"$${x}"`, tokens: `HEAD("$") IDENT(x) TAIL("")`},
	{id: "L39", src: `"\$"`, tokens: `STRING("$")`},
	{id: "L40", src: `"\${x}"`, tokens: `STRING("${x}")`},
	{id: "L41", src: `"a\${b}${c}"`, tokens: `HEAD("a${b}") IDENT(c) TAIL("")`},
	{id: "L42", src: `"\\${x}"`, tokens: `HEAD("\\") IDENT(x) TAIL("")`},
	{id: "L43", src: `"${}"`, tokens: `HEAD("") INVALID TAIL("")`, diags: []string{"L006@1:2"}},
	{id: "L44", src: `"${x`, tokens: `HEAD("") IDENT(x) INVALID`, diags: []string{"L007@1:2"}},
	{id: "L45", src: `"${x"`, tokens: `HEAD("") IDENT(x) INVALID`, diags: []string{"L003@1:5"}},
	{id: "L46", src: "\"${x\n}\"", tokens: `HEAD("") IDENT(x) INVALID NEWLINE } INVALID`, diags: []string{"L007@1:2", "L003@2:2"}},

	// Invalid input and encoding.
	{id: "L47", src: "\"\xff\"", tokens: "STRING(\"\xef\xbf\xbd\")", diags: []string{"L002@1:2"}},
	{id: "L48", src: "a \xff b", tokens: "IDENT(a) IDENT(b)", diags: []string{"L002@1:3"}},
	{id: "L49", src: "// \xff\xfe\nx", tokens: "NEWLINE IDENT(x)", diags: []string{"L002@1:4"}},
	{id: "L50", src: ";", tokens: "", diags: []string{"L001@1:1"}},
	{id: "L51", src: "@ # & |", tokens: "", diags: []string{"L001@1:1", "L001@1:3", "L001@1:5", "L001@1:7"}},
	{id: "L52", src: "x.y", tokens: "IDENT(x) . IDENT(y)"}, // ADR-0013: '.' is a token
	{id: "L53", src: "", tokens: "", checks: []tokCheck{{idx: -1, span: "[0,0)", pos: "1:1"}}},

	// Operators.
	{id: "L54", src: "<= >= == != && || -> ! = < > + - * / %", tokens: "<= >= == != && || -> ! = < > + - * / %"},
	{id: "L55", src: "a-b", tokens: "IDENT(a) - IDENT(b)"},
	{id: "L56", src: "-5", tokens: "- INT(5)"},

	// Numeric literals (ADR-0004 item 1).
	{id: "L57", src: "0 7 10 9223372036854775807", tokens: "INT(0) INT(7) INT(10) INT(9223372036854775807)"},
	{id: "L58", src: "9223372036854775808 99999999999999999999999", tokens: "INT(9223372036854775808) INT(99999999999999999999999)"},
	{id: "L59", src: "0.0 0.5 3.14 10.25 1.050", tokens: "FLOAT(0.0) FLOAT(0.5) FLOAT(3.14) FLOAT(10.25) FLOAT(1.050)"},
	{id: "L60", src: "007", tokens: "INVALID", diags: []string{"L009@1:1[0,3)"}},
	{id: "L61", src: "00 01.5 00.5", tokens: "INVALID INVALID INVALID", diags: []string{"L009@1:1", "L009@1:4", "L009@1:9"}},
	{id: "L62a", src: "1.", tokens: "INVALID", diags: []string{"L008@1:1[0,2)"}},
	{id: "L62b", src: "0.", tokens: "INVALID", diags: []string{"L008@1:1[0,2)"}},
	{id: "L63", src: ".5", tokens: "INVALID", diags: []string{"L008@1:1[0,2)"}},
	{id: "L64", src: "9abc", tokens: "INVALID", diags: []string{"L008@1:1[0,4)"}},
	{id: "L65a", src: "007abc", tokens: "INVALID", diags: []string{"L008@1:1[0,6)"}},
	{id: "L65b", src: "0abc", tokens: "INVALID", diags: []string{"L008@1:1[0,4)"}},
	{id: "L66a", src: "1.2.3", tokens: "INVALID", diags: []string{"L008@1:1[0,5)"}},
	// ADR-0012: a number stops before "..", so 1..2 is a range.
	{id: "L66b", src: "1..2", tokens: "INT(1) .. INT(2)"},
	{id: "L66c", src: "1.5.", tokens: "INVALID", diags: []string{"L008@1:1[0,4)"}},
	{id: "L67", src: "1e5 1_000 0x1F 1.5f", tokens: "INVALID INVALID INVALID INVALID", diags: []string{"L008@1:1", "L008@1:5", "L008@1:11", "L008@1:16"}},
	{id: "L68", src: "1+.5", tokens: "INT(1) + INVALID", diags: []string{"L008@1:3[2,4)"}},

	// Loops (ADR-0012).
	{id: "L70", src: "while for in break continue", tokens: "while for in break continue"},
	{id: "L71", src: "0..n a..b", tokens: "INT(0) .. IDENT(n) IDENT(a) .. IDENT(b)"},
	{id: "L72", src: "1.5..2 0 .. 10", tokens: "FLOAT(1.5) .. INT(2) INT(0) .. INT(10)"},
	{id: "L73", src: "1...2", tokens: "INT(1) .. INVALID", diags: []string{"L008@1:4[3,5)"}},
	{id: "L74", src: "...", tokens: ".. ."},
	{id: "L75", src: "007..1", tokens: "INVALID .. INT(1)", diags: []string{"L009@1:1[0,3)"}},

	// Collections, enums and pattern matching (ADR-0013).
	{id: "L76", src: "enum match", tokens: "enum match"},
	{id: "L77", src: "[1, 2] a[0] [:]", tokens: "[ INT(1) , INT(2) ] IDENT(a) [ INT(0) ] [ : ]"},
	{id: "L78", src: "xs.len() Shape.Circle", tokens: "IDENT(xs) . IDENT(len) ( ) IDENT(Shape) . IDENT(Circle)"},
	{id: "L79", src: "=> = == =>=", tokens: "=> = == => ="},
	{id: "L80", src: "a.5 0..n.len", tokens: "IDENT(a) INVALID INT(0) .. IDENT(n) . IDENT(len)", diags: []string{"L008@1:2"}},
}

func describeTokens(toks []token.Token) string {
	var parts []string
	for _, t := range toks {
		switch t.Kind {
		case token.EOF:
			continue
		case token.Ident:
			parts = append(parts, "IDENT("+t.Lexeme+")")
		case token.Int:
			parts = append(parts, "INT("+t.Lexeme+")")
		case token.Float:
			parts = append(parts, "FLOAT("+t.Lexeme+")")
		case token.Reserved:
			parts = append(parts, "RESERVED("+t.Lexeme+")")
		case token.String:
			parts = append(parts, fmt.Sprintf("STRING(%q)", t.Value))
		case token.TemplateHead:
			parts = append(parts, fmt.Sprintf("HEAD(%q)", t.Value))
		case token.TemplateMiddle:
			parts = append(parts, fmt.Sprintf("MIDDLE(%q)", t.Value))
		case token.TemplateTail:
			parts = append(parts, fmt.Sprintf("TAIL(%q)", t.Value))
		default:
			parts = append(parts, t.Kind.String())
		}
	}
	return strings.Join(parts, " ")
}

func describeDiag(f *source.File, d diagnostic.Diagnostic, withSpan bool) string {
	p := f.Position(d.Span.Start)
	s := fmt.Sprintf("%s@%d:%d", strings.TrimPrefix(string(d.Code), "TR-"), p.Line, p.Column)
	if withSpan {
		s += fmt.Sprintf("[%d,%d)", d.Span.Start, d.Span.End)
	}
	return s
}

func TestLexRequiredCases(t *testing.T) {
	for _, tc := range lexCases {
		t.Run(tc.id, func(t *testing.T) {
			f := source.NewFile(1, "t.tr", []byte(tc.src))
			toks, diags := Lex(f)
			checkInvariants(t, f, toks, diags)

			if got := describeTokens(toks); got != tc.tokens {
				t.Errorf("tokens:\n got  %s\n want %s", got, tc.tokens)
			}
			var got []string
			for i, d := range diags {
				withSpan := i < len(tc.diags) && strings.Contains(tc.diags[i], "[")
				got = append(got, describeDiag(f, d, withSpan))
			}
			if strings.Join(got, " ") != strings.Join(tc.diags, " ") {
				t.Errorf("diagnostics:\n got  %v\n want %v", got, tc.diags)
			}
			for _, c := range tc.checks {
				tok := toks[len(toks)-1]
				if c.idx >= 0 {
					tok = toks[c.idx]
				}
				if c.span != "" {
					if s := fmt.Sprintf("[%d,%d)", tok.Span.Start, tok.Span.End); s != c.span {
						t.Errorf("token %d span = %s, want %s", c.idx, s, c.span)
					}
				}
				if c.pos != "" {
					p := f.Position(tok.Span.Start)
					if s := fmt.Sprintf("%d:%d", p.Line, p.Column); s != c.pos {
						t.Errorf("token %d position = %s, want %s", c.idx, s, c.pos)
					}
				}
			}
		})
	}
}

// checkInvariants verifies the token-stream contract of docs/03 §3.3.
func checkInvariants(t *testing.T, f *source.File, toks []token.Token, diags []diagnostic.Diagnostic) {
	t.Helper()
	if len(toks) == 0 || toks[len(toks)-1].Kind != token.EOF {
		t.Fatalf("token stream must end with EOF")
	}
	if eof := toks[len(toks)-1].Span; eof.Start != f.Len() || eof.End != f.Len() {
		t.Errorf("EOF span = %v, want empty span at %d", eof, f.Len())
	}
	prevEnd := 0
	for i, tok := range toks {
		s := tok.Span
		if s.Start < prevEnd || s.End < s.Start || s.End > f.Len() {
			t.Fatalf("token %d (%v) span %v violates ordering/bounds (prev end %d)", i, tok.Kind, s, prevEnd)
		}
		if tok.Kind == token.EOF && i != len(toks)-1 {
			t.Fatalf("EOF before end of stream")
		}
		if tok.Lexeme != string(f.Content()[s.Start:s.End]) {
			t.Fatalf("token %d lexeme %q does not match its span", i, tok.Lexeme)
		}
		prevEnd = s.End
	}
	for _, d := range diags {
		if d.Span.Start < 0 || d.Span.End > f.Len() || d.Span.Start > d.Span.End {
			t.Fatalf("diagnostic %s span %v out of bounds", d.Code, d.Span)
		}
		if d.Code.Phase() != 'L' {
			t.Fatalf("lexer produced non-lexer code %s", d.Code)
		}
	}
}

// TestEveryLexerCodeIsTested enforces docs/08 §8.1: each lexer code in the
// registry is produced by at least one required case.
func TestEveryLexerCodeIsTested(t *testing.T) {
	produced := map[diagnostic.Code]bool{}
	for _, tc := range lexCases {
		_, diags := Lex(source.NewFile(1, "t", []byte(tc.src)))
		for _, d := range diags {
			produced[d.Code] = true
		}
	}
	var missing []string
	for _, c := range diagnostic.Registry() {
		if c.Phase() == 'L' && !produced[c] {
			missing = append(missing, string(c))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("lexer codes never produced by a required case: %v", missing)
	}
}

func TestDeterministic(t *testing.T) {
	src := "fn main() {\r\n\tlet s = \"${a}\xff\" 9abc ; é\r}"
	a, da := Lex(source.NewFile(1, "t", []byte(src)))
	b, db := Lex(source.NewFile(1, "t", []byte(src)))
	if fmt.Sprint(a) != fmt.Sprint(b) || fmt.Sprint(da) != fmt.Sprint(db) {
		t.Error("lexing the same input twice gave different results")
	}
}

func TestDiagnosticHelpText(t *testing.T) {
	_, diags := Lex(source.NewFile(1, "t", []byte("; é")))
	if len(diags) != 2 || diags[0].Help == "" || diags[1].Help == "" {
		t.Errorf("expected help text on ';' and non-ASCII diagnostics, got %+v", diags)
	}
}

// TestLexRandomInputs complements the fuzz target (L69) with a fixed-seed
// sweep so that robustness is exercised on every `go test` run.
func TestLexRandomInputs(t *testing.T) {
	alphabet := []string{
		"a", "Z", "_", "0", "7", ".", "\"", "\\", "$", "{", "}", "(", ")", "/", "*",
		"\n", "\r", "\t", " ", "-", ">", "=", "!", "&", "|", ";", "é", "日", "\xff",
		bom, "${", "/*", "*/", "//", "\\$", "1.5", "007", "let", "if", "else",
	}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		n := rng.Intn(24)
		for j := 0; j < n; j++ {
			b.WriteString(alphabet[rng.Intn(len(alphabet))])
		}
		src := b.String()
		f := source.NewFile(1, "t", []byte(src))
		toks, diags := Lex(f)
		checkInvariants(t, f, toks, diags)
		if t.Failed() {
			t.Fatalf("input %q", src)
		}
	}
}
