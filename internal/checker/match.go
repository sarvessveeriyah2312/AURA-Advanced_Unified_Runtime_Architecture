package checker

import (
	"strconv"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
)

// checkMatch checks a match statement (ADR-0013): each pattern against the
// subject's type, each arm's body in a scope holding the pattern's
// bindings, unreachable arms (T028) and exhaustiveness (T027).
func (c *checker) checkMatch(s *ast.MatchStmt) {
	subject := c.expr(s.Subject)
	analyse := subject != Invalid && !subject.containsInvalid()
	var rows [][]space
	for _, arm := range s.Arms {
		if arm == nil {
			analyse = false
			continue
		}
		saved := c.scope
		c.scope = newScope(saved)
		sp, ok := c.checkPattern(arm.Pattern, subject)
		if !ok {
			analyse = false
		}
		if analyse {
			if !useful(rows, []space{sp}, []Type{subject}) {
				c.diags = append(c.diags, diagnostic.Errorf(diagnostic.UnreachablePattern, arm.Pattern.Span(),
					"this arm can never run: the arms above already match every value it matches").
					WithHelp("remove it, or move it above the arm that covers it"))
			}
			rows = append(rows, []space{sp})
		}
		c.checkBlock(arm.Body)
		c.scope = saved
	}
	if !analyse {
		// Errors are already reported; treat the match as exhaustive so
		// they do not cascade into T008.
		c.out.Exhaustive[s] = true
		return
	}
	if w, missing := witness(rows, []Type{subject}); missing {
		c.diags = append(c.diags, diagnostic.Errorf(diagnostic.NonExhaustiveMatch, s.Subject.Span(),
			"this match does not cover every %s value: %s is not matched", subject, w[0]).
			WithHelp("add an arm for it, or a '_' arm to match everything else"))
		return
	}
	c.out.Exhaustive[s] = true
}

// space is the set of values a pattern matches, for the usefulness
// algorithm: a wildcard, or a constructor (a variant, or a literal) with
// sub-spaces for its fields.
type space struct {
	wild bool
	ctor ctor
	args []space
}

// ctor is a constructor of a type: an enum variant, true or false, or an
// Int or String literal (an infinite family).
type ctor struct {
	id     string
	label  string
	fields []Type
}

func (s space) String() string {
	if s.wild {
		return "_"
	}
	if len(s.args) == 0 {
		return s.ctor.label
	}
	parts := make([]string, len(s.args))
	for i, a := range s.args {
		parts[i] = a.String()
	}
	return s.ctor.label + "(" + strings.Join(parts, ", ") + ")"
}

// checkPattern checks pattern p against a value of type t, declaring its
// bindings in the current scope. It returns the pattern's space and
// whether the pattern is valid.
func (c *checker) checkPattern(p ast.Pattern, t Type) (space, bool) {
	switch p := p.(type) {
	case *ast.WildcardPat:
		return space{wild: true}, true
	case *ast.BindPat:
		sym := &Symbol{Name: p.Name.Name, Kind: MatchVarSymbol, Type: t, Decl: p.Name.Loc}
		if c.declare(c.scope, sym, "pattern variable") {
			c.out.Defs[p.Name] = sym
		}
		return space{wild: true}, true
	case *ast.LitPat:
		return c.checkLitPattern(p, t)
	case *ast.VariantPat:
		return c.checkVariantPattern(p, t)
	}
	return space{wild: true}, false
}

func (c *checker) checkLitPattern(p *ast.LitPat, t Type) (space, bool) {
	lt := c.expr(p.Value)
	var k ctor
	switch v := p.Value.(type) {
	case *ast.FloatLit:
		c.errorf(diagnostic.InvalidPattern, p.Span(), "Float values cannot be used as patterns; compare them with 'if' instead")
		return space{}, false
	case *ast.UnaryExpr:
		if _, ok := v.Operand.(*ast.FloatLit); ok {
			c.errorf(diagnostic.InvalidPattern, p.Span(), "Float values cannot be used as patterns; compare them with 'if' instead")
			return space{}, false
		}
		n, ok := c.out.IntValues[v]
		if !ok {
			if inner, isInt := v.Operand.(*ast.IntLit); isInt {
				if m, ok := c.out.IntValues[inner]; ok {
					n, ok = -m, true
					c.out.IntValues[v] = n
				}
			}
		}
		k = ctor{id: "i:" + strconv.FormatInt(n, 10), label: strconv.FormatInt(n, 10)}
	case *ast.IntLit:
		n := c.out.IntValues[v]
		k = ctor{id: "i:" + strconv.FormatInt(n, 10), label: strconv.FormatInt(n, 10)}
	case *ast.StringLit:
		k = ctor{id: "s:" + v.Value, label: strconv.Quote(v.Value)}
	case *ast.BoolLit:
		k = ctor{id: "b:" + strconv.FormatBool(v.Value), label: strconv.FormatBool(v.Value)}
	default:
		return space{}, false
	}
	if lt == Invalid {
		return space{}, false
	}
	if t != Invalid && !compatible(t, lt) {
		c.errorf(diagnostic.TypeMismatch, p.Span(), "this pattern is %s but the value being matched is %s", lt, t)
		return space{}, false
	}
	return space{ctor: k}, true
}

func (c *checker) checkVariantPattern(p *ast.VariantPat, t Type) (space, bool) {
	ok := true
	bindAll := func() {
		for _, a := range p.Args {
			c.checkPattern(a, Invalid)
		}
	}
	en := c.enums[p.Enum.Name]
	if en == nil {
		c.errorf(diagnostic.UnknownVariant, p.Enum.Loc, "unknown enum '%s'", p.Enum.Name)
		bindAll()
		return space{}, false
	}
	v := en.Lookup(p.Variant.Name)
	if v == nil {
		c.errorf(diagnostic.UnknownVariant, p.Variant.Loc, "enum '%s' has no variant '%s'", en.Name, p.Variant.Name)
		bindAll()
		return space{}, false
	}
	c.out.Variants[p] = v
	name := en.Name + "." + v.Name
	fieldTypes := make([]Type, len(v.Fields))
	if t != Invalid && (t.Kind != KEnum || t.Enum != en) {
		c.errorf(diagnostic.TypeMismatch, p.Loc, "the pattern %s cannot match a value of type %s", name, t)
		ok = false
	}
	args := enumArgs(t)
	for i, f := range v.Fields {
		fieldTypes[i] = Invalid
		if ok && t != Invalid {
			fieldTypes[i] = subst(f, args)
		}
	}
	switch {
	case p.Args == nil && len(v.Fields) > 0:
		blanks := strings.TrimSuffix(strings.Repeat("_, ", len(v.Fields)), ", ")
		c.errorf(diagnostic.InvalidPattern, p.Loc, "%s has %d field(s); write %s(%s) to match it", name, len(v.Fields), name, blanks)
		return space{}, false
	case p.Args != nil && len(v.Fields) == 0:
		c.errorf(diagnostic.InvalidPattern, p.Loc, "%s has no fields; write it without parentheses", name)
		bindAll()
		return space{}, false
	case len(p.Args) != len(v.Fields):
		c.errorf(diagnostic.InvalidPattern, p.Loc, "%s has %d field(s) but the pattern has %d", name, len(v.Fields), len(p.Args))
		bindAll()
		return space{}, false
	}
	sp := space{ctor: ctor{id: "v" + strconv.Itoa(v.Index), label: name, fields: fieldTypes}}
	for i, a := range p.Args {
		sub, subOK := c.checkPattern(a, fieldTypes[i])
		ok = ok && subOK
		sp.args = append(sp.args, sub)
	}
	return sp, ok
}

// ---------------------------------------------------------------------------
// Usefulness (Maranget, "Warnings for pattern matching", 2007)

// ctorsOf returns every constructor of t, and whether the list is
// complete. Int, String, Float and type parameters have infinitely many
// values: only a wildcard covers them.
func ctorsOf(t Type) ([]ctor, bool) {
	switch t.Kind {
	case KBool:
		return []ctor{{id: "b:true", label: "true"}, {id: "b:false", label: "false"}}, true
	case KEnum:
		args := enumArgs(t)
		out := make([]ctor, len(t.Enum.Variants))
		for i, v := range t.Enum.Variants {
			fields := make([]Type, len(v.Fields))
			for j, f := range v.Fields {
				fields[j] = subst(f, args)
			}
			out[i] = ctor{id: "v" + strconv.Itoa(v.Index), label: t.Enum.Name + "." + v.Name, fields: fields}
		}
		return out, true
	}
	return nil, false
}

func wilds(n int) []space {
	out := make([]space, n)
	for i := range out {
		out[i] = space{wild: true}
	}
	return out
}

func concat(a, b []space) []space { return append(append([]space(nil), a...), b...) }

func concatTypes(a, b []Type) []Type { return append(append([]Type(nil), a...), b...) }

// specialize keeps the rows whose first column can match constructor k,
// replacing that column with k's fields.
func specialize(rows [][]space, k ctor) [][]space {
	var out [][]space
	for _, row := range rows {
		h := row[0]
		switch {
		case h.wild:
			out = append(out, concat(wilds(len(k.fields)), row[1:]))
		case h.ctor.id == k.id:
			out = append(out, concat(h.args, row[1:]))
		}
	}
	return out
}

// defaults keeps the rows whose first column is a wildcard, without it.
func defaults(rows [][]space) [][]space {
	var out [][]space
	for _, row := range rows {
		if row[0].wild {
			out = append(out, row[1:])
		}
	}
	return out
}

// headCtors returns the constructor IDs used in the first column.
func headCtors(rows [][]space) map[string]bool {
	used := map[string]bool{}
	for _, row := range rows {
		if !row[0].wild {
			used[row[0].ctor.id] = true
		}
	}
	return used
}

func coversAll(used map[string]bool, all []ctor) bool {
	for _, k := range all {
		if !used[k.id] {
			return false
		}
	}
	return true
}

// useful reports whether the pattern vector q matches some value that no
// row of rows matches.
func useful(rows [][]space, q []space, types []Type) bool {
	if len(q) == 0 {
		return len(rows) == 0
	}
	head := q[0]
	if !head.wild {
		k := head.ctor
		return useful(specialize(rows, k), concat(head.args, q[1:]), concatTypes(k.fields, types[1:]))
	}
	all, complete := ctorsOf(types[0])
	if complete && coversAll(headCtors(rows), all) {
		for _, k := range all {
			if useful(specialize(rows, k), concat(wilds(len(k.fields)), q[1:]), concatTypes(k.fields, types[1:])) {
				return true
			}
		}
		return false
	}
	return useful(defaults(rows), q[1:], types[1:])
}

// witness returns a vector of patterns, one per type, matching a value no
// row matches, and whether one exists. It names a missing case in T027.
func witness(rows [][]space, types []Type) ([]space, bool) {
	if len(types) == 0 {
		if len(rows) == 0 {
			return []space{}, true
		}
		return nil, false
	}
	used := headCtors(rows)
	all, complete := ctorsOf(types[0])
	if complete && coversAll(used, all) {
		for _, k := range all {
			n := len(k.fields)
			if w, ok := witness(specialize(rows, k), concatTypes(k.fields, types[1:])); ok {
				return concat([]space{{ctor: k, args: w[:n]}}, w[n:]), true
			}
		}
		return nil, false
	}
	w, ok := witness(defaults(rows), types[1:])
	if !ok {
		return nil, false
	}
	head := space{wild: true}
	if complete && len(used) > 0 {
		for _, k := range all {
			if !used[k.id] {
				head = space{ctor: k, args: wilds(len(k.fields))}
				break
			}
		}
	}
	return concat([]space{head}, w), true
}
