package playground

import "strings"

// PrettyAST indents the one-line S-expression of ast.Format: a list that
// fits in maxFlat characters stays on one line; a longer one keeps its
// head and leading atoms on the first line and puts each remaining
// element on its own indented line.
func PrettyAST(sexpr string) string {
	n, _ := parseSexpr(sexpr, 0)
	var b strings.Builder
	n.write(&b, 0)
	b.WriteByte('\n')
	return b.String()
}

const maxFlat = 60

// sexpr is an atom (list == nil) or a list.
type sexpr struct {
	atom string
	list []sexpr
	flat string
}

// parseSexpr reads one atom or list starting at i. Atoms are runs of
// non-space, non-paren characters, or Go-quoted strings ("..." with
// backslash escapes), so parentheses inside strings are not structure.
func parseSexpr(s string, i int) (sexpr, int) {
	for i < len(s) && s[i] == ' ' {
		i++
	}
	if i >= len(s) {
		return sexpr{atom: ""}, i
	}
	if s[i] != '(' {
		start := i
		if s[i] == '"' {
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' {
					i++
				}
				i++
			}
			i++
		} else {
			for i < len(s) && s[i] != ' ' && s[i] != '(' && s[i] != ')' {
				i++
			}
		}
		if i > len(s) {
			i = len(s)
		}
		return sexpr{atom: s[start:i]}, i
	}
	start := i
	i++ // '('
	var items []sexpr
	for i < len(s) {
		for i < len(s) && s[i] == ' ' {
			i++
		}
		if i >= len(s) {
			break
		}
		if s[i] == ')' {
			i++
			break
		}
		var item sexpr
		item, i = parseSexpr(s, i)
		items = append(items, item)
	}
	if items == nil {
		items = []sexpr{}
	}
	return sexpr{list: items, flat: s[start:i]}, i
}

func (n sexpr) write(b *strings.Builder, indent int) {
	if n.list == nil {
		b.WriteString(n.atom)
		return
	}
	if len(n.flat) <= maxFlat || len(n.list) == 0 {
		b.WriteString(n.flat)
		return
	}
	// Leading atoms (and empty lists such as "()") stay on the head line:
	// "(fn main () -> Unit" then the body.
	b.WriteByte('(')
	head := 0
	for head < len(n.list) && (n.list[head].list == nil || len(n.list[head].list) == 0) {
		if head > 0 {
			b.WriteByte(' ')
		}
		n.list[head].write(b, indent+1)
		head++
	}
	for i, item := range n.list[head:] {
		if head == 0 && i == 0 {
			item.write(b, indent+1)
			continue
		}
		b.WriteByte('\n')
		b.WriteString(strings.Repeat("  ", indent+1))
		item.write(b, indent+1)
	}
	b.WriteByte(')')
}
