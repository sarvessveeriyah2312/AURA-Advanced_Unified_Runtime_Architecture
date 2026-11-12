package token

import "github.com/sarvessveeriyah2312/Tread/internal/source"

// Kind is the kind of a lexical token.
type Kind int

// Token kinds. The order of the keyword block is significant: IsKeyword
// relies on it.
const (
	EOF     Kind = iota
	Newline      // LF, CRLF or lone CR (ADR-0002 D1, ADR-0003 L2)
	Invalid      // malformed input already reported by the lexer
	Ident
	Int            // lexeme preserved; never converted by the lexer
	Float          // lexeme preserved; never converted by the lexer
	String         // string literal without interpolation
	TemplateHead   // "text${
	TemplateMiddle // }text${
	TemplateTail   // }text"
	Reserved       // reserved word (docs/02 §2.4)

	keywordStart
	Module
	Fn
	Let
	Mut
	If
	Else
	True
	False
	Return
	keywordEnd

	Plus    // +
	Minus   // -
	Star    // *
	Slash   // /
	Percent // %
	Eq      // ==
	Ne      // !=
	Lt      // <
	Le      // <=
	Gt      // >
	Ge      // >=
	AndAnd  // &&
	OrOr    // ||
	Not     // !
	Assign  // =
	Arrow   // ->
	LParen  // (
	RParen  // )
	LBrace  // {
	RBrace  // }
	Comma   // ,
	Colon   // :
)

var names = map[Kind]string{
	EOF: "EOF", Newline: "NEWLINE", Invalid: "INVALID", Ident: "IDENT",
	Int: "INT", Float: "FLOAT", String: "STRING",
	TemplateHead: "TEMPLATE_HEAD", TemplateMiddle: "TEMPLATE_MIDDLE", TemplateTail: "TEMPLATE_TAIL",
	Reserved: "RESERVED",
	Module:   "module", Fn: "fn", Let: "let", Mut: "mut", If: "if", Else: "else",
	True: "true", False: "false", Return: "return",
	Plus: "+", Minus: "-", Star: "*", Slash: "/", Percent: "%",
	Eq: "==", Ne: "!=", Lt: "<", Le: "<=", Gt: ">", Ge: ">=",
	AndAnd: "&&", OrOr: "||", Not: "!", Assign: "=", Arrow: "->",
	LParen: "(", RParen: ")", LBrace: "{", RBrace: "}", Comma: ",", Colon: ":",
}

// String returns the keyword or operator spelling, or an upper-case name
// for other kinds.
func (k Kind) String() string {
	if s, ok := names[k]; ok {
		return s
	}
	return "Kind(?)"
}

// IsKeyword reports whether k is an MVP keyword (docs/02 §2.4).
func (k Kind) IsKeyword() bool { return k > keywordStart && k < keywordEnd }

var keywords = map[string]Kind{
	"module": Module, "fn": Fn, "let": Let, "mut": Mut, "if": If,
	"else": Else, "true": True, "false": False, "return": Return,
}

var reserved = map[string]bool{
	"class": true, "interface": true, "enum": true, "match": true, "for": true,
	"while": true, "async": true, "await": true, "import": true, "pub": true,
}

// Lookup classifies an identifier-shaped lexeme as a keyword, a reserved
// word, or Ident. Built-in type names such as Int are identifiers
// (ADR-0004 item 3).
func Lookup(lexeme string) Kind {
	if k, ok := keywords[lexeme]; ok {
		return k
	}
	if reserved[lexeme] {
		return Reserved
	}
	return Ident
}

// Token is one lexical token.
type Token struct {
	Kind Kind
	Span source.Span
	// Lexeme is the exact source text of the token.
	Lexeme string
	// Value is the decoded text of String and Template* tokens, with
	// escapes processed and delimiters removed. Empty for other kinds.
	Value string
}
