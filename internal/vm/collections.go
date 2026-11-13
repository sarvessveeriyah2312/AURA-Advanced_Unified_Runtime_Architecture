package vm

import (
	"strconv"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/bytecode"
)

// Collections, enums and generic values (ADR-0013).
//
// Arrays and maps have value semantics, implemented with copy-on-write.
// A heap value is marked shared whenever it may gain a second owner:
// when it is loaded from or stored into a variable, put into a
// container, or read out of one. A change through a variable path
// (a[i] = v, a.push(x), m.remove(k)) first replaces every shared node on
// the path with an unshared clone, so changes never leak into another
// owner, and repeated changes to the same variable happen in place.

// optionEnum is the module index of the built-in Option<T>; Some is
// variant 0 and None variant 1.
const optionEnum = 0

func some(v bytecode.Value) bytecode.Value {
	bytecode.MarkShared(v)
	return bytecode.EnumOf(optionEnum, 0, []bytecode.Value{v})
}

func none() bytecode.Value { return bytecode.EnumOf(optionEnum, 1, nil) }

// popN removes and returns the top n values, oldest first.
func (m *machine) popN(n int) ([]bytecode.Value, *Error) {
	base := m.frames[len(m.frames)-1].stackBase
	if n < 0 || len(m.stack)-base < n {
		return nil, m.invalid("stack underflow: need %d values", n)
	}
	vals := make([]bytecode.Value, n)
	copy(vals, m.stack[len(m.stack)-n:])
	m.stack = m.stack[:len(m.stack)-n]
	return vals, nil
}

func (m *machine) array(v bytecode.Value) (*bytecode.Array, *Error) {
	if a := v.AsArray(); v.Kind == bytecode.KindArray && a != nil {
		return a, nil
	}
	return nil, m.invalid("expected Array, found %s", v.Kind)
}

func (m *machine) mapOf(v bytecode.Value) (*bytecode.Map, *Error) {
	if mp := v.AsMap(); v.Kind == bytecode.KindMap && mp != nil {
		return mp, nil
	}
	return nil, m.invalid("expected Map, found %s", v.Kind)
}

func (m *machine) key(v bytecode.Value) (bytecode.MapKey, *Error) {
	k, ok := bytecode.KeyOf(v)
	if !ok {
		return k, m.invalid("a %s value cannot be a map key", v.Kind)
	}
	return k, nil
}

// position checks an array index.
func (m *machine) position(a *bytecode.Array, k bytecode.Value) (int, *Error) {
	if k.Kind != bytecode.KindInt {
		return 0, m.invalid("array index is %s, not Int", k.Kind)
	}
	if k.Int < 0 || k.Int >= int64(len(a.Elems)) {
		return 0, m.fail(CodeIndexOutOfRange, "index %d is out of range for an array of length %d", k.Int, len(a.Elems))
	}
	return int(k.Int), nil
}

func (m *machine) missingKey(k bytecode.Value) *Error {
	return m.fail(CodeKeyNotFound, "key %s is not in the map", quoteValue(m.mod, k))
}

// fieldOf checks a struct field index, given as an Int key in a path
// (ADR-0014).
func (m *machine) fieldOf(v bytecode.Value, k bytecode.Value) (*bytecode.StructValue, int, *Error) {
	s := v.AsStruct()
	if v.Kind != bytecode.KindStruct || s == nil {
		return nil, 0, m.invalid("expected Struct, found %s", v.Kind)
	}
	if k.Kind != bytecode.KindInt || k.Int < 0 || k.Int >= int64(len(s.Fields)) {
		return nil, 0, m.invalid("field %s of a struct with %d fields", Format(k), len(s.Fields))
	}
	return s, int(k.Int), nil
}

// elem returns container[k] without marking it.
func (m *machine) elem(c, k bytecode.Value) (bytecode.Value, *Error) {
	switch c.Kind {
	case bytecode.KindArray:
		a, e := m.array(c)
		if e != nil {
			return c, e
		}
		i, e := m.position(a, k)
		if e != nil {
			return c, e
		}
		return a.Elems[i], nil
	case bytecode.KindMap:
		mp, e := m.mapOf(c)
		if e != nil {
			return c, e
		}
		key, e := m.key(k)
		if e != nil {
			return c, e
		}
		i, ok := mp.Find(key)
		if !ok {
			return c, m.missingKey(k)
		}
		return mp.Vals[i], nil
	case bytecode.KindStruct:
		s, i, e := m.fieldOf(c, k)
		if e != nil {
			return c, e
		}
		return s.Fields[i], nil
	}
	return c, m.invalid("cannot index a %s value", c.Kind)
}

// readPath returns local[slot][k1]...[kn].
func (m *machine) readPath(fr *frame, slot int, keys []bytecode.Value) (bytecode.Value, *Error) {
	v := fr.locals[slot]
	for _, k := range keys {
		var e *Error
		if v, e = m.elem(v, k); e != nil {
			return v, e
		}
	}
	return v, nil
}

// unshare replaces a shared array or map at ref with an unshared clone.
func unshare(ref *bytecode.Value) {
	switch r := ref.Ref.(type) {
	case *bytecode.Array:
		if r.Shared {
			*ref = bytecode.Clone(*ref)
		}
	case *bytecode.Map:
		if r.Shared {
			*ref = bytecode.Clone(*ref)
		}
	case *bytecode.StructValue:
		if r.Shared {
			*ref = bytecode.Clone(*ref)
		}
	}
}

// writeRef returns a pointer to local[slot][k1]...[kn], unsharing every
// node on the path, including the last (copy-on-write).
func (m *machine) writeRef(fr *frame, slot int, keys []bytecode.Value) (*bytecode.Value, *Error) {
	ref := &fr.locals[slot]
	unshare(ref)
	for _, k := range keys {
		switch ref.Kind {
		case bytecode.KindArray:
			a, e := m.array(*ref)
			if e != nil {
				return nil, e
			}
			i, e := m.position(a, k)
			if e != nil {
				return nil, e
			}
			ref = &a.Elems[i]
		case bytecode.KindMap:
			mp, e := m.mapOf(*ref)
			if e != nil {
				return nil, e
			}
			key, e := m.key(k)
			if e != nil {
				return nil, e
			}
			i, ok := mp.Find(key)
			if !ok {
				return nil, m.missingKey(k)
			}
			ref = &mp.Vals[i]
		case bytecode.KindStruct:
			s, i, e := m.fieldOf(*ref, k)
			if e != nil {
				return nil, e
			}
			ref = &s.Fields[i]
		default:
			return nil, m.invalid("cannot index a %s value", ref.Kind)
		}
		unshare(ref)
	}
	return ref, nil
}

// collectionOp executes the ADR-0013 instructions. It reports handled
// false for any other opcode.
func (m *machine) collectionOp(fr *frame, in bytecode.Instr) (handled bool, err error) {
	var e *Error
	switch in.Op {
	case bytecode.OpMakeArray:
		var elems []bytecode.Value
		if elems, e = m.popN(in.A); e == nil {
			for _, v := range elems {
				bytecode.MarkShared(v)
			}
			m.push(bytecode.ArrayValue(elems))
		}
	case bytecode.OpMakeMap:
		var kv []bytecode.Value
		if kv, e = m.popN(2 * in.A); e == nil {
			mv := bytecode.NewMap()
			mp := mv.AsMap()
			for i := 0; i < len(kv) && e == nil; i += 2 {
				var k bytecode.MapKey
				if k, e = m.key(kv[i]); e == nil {
					bytecode.MarkShared(kv[i+1])
					mp.Set(kv[i], k, kv[i+1])
				}
			}
			if e == nil {
				m.push(mv)
			}
		}
	case bytecode.OpMakeEnum:
		var fields []bytecode.Value
		if fields, e = m.popN(m.mod.Enums[in.A].Variants[in.B].Fields); e == nil {
			for _, v := range fields {
				bytecode.MarkShared(v)
			}
			m.push(bytecode.EnumOf(in.A, in.B, fields))
		}
	case bytecode.OpLoadPath, bytecode.OpPeekPath:
		var keys []bytecode.Value
		if keys, e = m.popN(in.B); e == nil {
			var v bytecode.Value
			if v, e = m.readPath(fr, in.A, keys); e == nil {
				if in.Op == bytecode.OpLoadPath {
					bytecode.MarkShared(v)
				}
				m.push(v)
			}
		}
	case bytecode.OpStorePath:
		var vals []bytecode.Value
		if vals, e = m.popN(in.B + 1); e != nil {
			break
		}
		keys, value := vals[:in.B-1], vals[in.B]
		last := vals[in.B-1]
		var ref *bytecode.Value
		if ref, e = m.writeRef(fr, in.A, keys); e != nil {
			break
		}
		bytecode.MarkShared(value)
		switch ref.Kind {
		case bytecode.KindArray:
			a := ref.AsArray()
			var i int
			if i, e = m.position(a, last); e == nil {
				a.Elems[i] = value
			}
		case bytecode.KindMap:
			var k bytecode.MapKey
			if k, e = m.key(last); e == nil {
				ref.AsMap().Set(last, k, value)
			}
		case bytecode.KindStruct:
			var s *bytecode.StructValue
			var i int
			if s, i, e = m.fieldOf(*ref, last); e == nil {
				s.Fields[i] = value
			}
		default:
			e = m.invalid("cannot index a %s value", ref.Kind)
		}
	case bytecode.OpPushPath:
		var vals []bytecode.Value
		if vals, e = m.popN(in.B + 1); e != nil {
			break
		}
		var ref *bytecode.Value
		if ref, e = m.writeRef(fr, in.A, vals[:in.B]); e != nil {
			break
		}
		var a *bytecode.Array
		if a, e = m.array(*ref); e == nil {
			bytecode.MarkShared(vals[in.B])
			a.Elems = append(a.Elems, vals[in.B])
			m.push(bytecode.UnitValue())
		}
	case bytecode.OpPopPath:
		var keys []bytecode.Value
		if keys, e = m.popN(in.B); e != nil {
			break
		}
		var ref *bytecode.Value
		if ref, e = m.writeRef(fr, in.A, keys); e != nil {
			break
		}
		var a *bytecode.Array
		if a, e = m.array(*ref); e == nil {
			if n := len(a.Elems); n == 0 {
				m.push(none())
			} else {
				last := a.Elems[n-1]
				a.Elems[n-1] = bytecode.Value{}
				a.Elems = a.Elems[:n-1]
				m.push(some(last))
			}
		}
	case bytecode.OpRemovePath:
		var vals []bytecode.Value
		if vals, e = m.popN(in.B + 1); e != nil {
			break
		}
		var ref *bytecode.Value
		if ref, e = m.writeRef(fr, in.A, vals[:in.B]); e != nil {
			break
		}
		var mp *bytecode.Map
		if mp, e = m.mapOf(*ref); e != nil {
			break
		}
		var k bytecode.MapKey
		if k, e = m.key(vals[in.B]); e == nil {
			if v, ok := mp.Delete(k); ok {
				m.push(some(v))
			} else {
				m.push(none())
			}
		}
	case bytecode.OpIndex:
		var v []bytecode.Value
		if v, e = m.popN(2); e == nil {
			var r bytecode.Value
			if r, e = m.elem(v[0], v[1]); e == nil {
				bytecode.MarkShared(r)
				m.push(r)
			}
		}
	case bytecode.OpLen:
		var v bytecode.Value
		if v, e = m.pop(); e == nil {
			switch v.Kind {
			case bytecode.KindArray:
				var a *bytecode.Array
				if a, e = m.array(v); e == nil {
					m.push(bytecode.IntValue(int64(len(a.Elems))))
				}
			case bytecode.KindMap:
				var mp *bytecode.Map
				if mp, e = m.mapOf(v); e == nil {
					m.push(bytecode.IntValue(int64(mp.Len())))
				}
			default:
				e = m.invalid("LEN of a %s value", v.Kind)
			}
		}
	case bytecode.OpContains:
		var v []bytecode.Value
		if v, e = m.popN(2); e != nil {
			break
		}
		switch v[0].Kind {
		case bytecode.KindArray:
			var a *bytecode.Array
			if a, e = m.array(v[0]); e == nil {
				found := false
				for _, x := range a.Elems {
					if bytecode.Equal(x, v[1]) {
						found = true
						break
					}
				}
				m.push(bytecode.BoolValue(found))
			}
		case bytecode.KindMap:
			mp, _ := m.mapOf(v[0])
			var k bytecode.MapKey
			if k, e = m.key(v[1]); e == nil {
				_, found := mp.Find(k)
				m.push(bytecode.BoolValue(found))
			}
		default:
			e = m.invalid("CONTAINS on a %s value", v[0].Kind)
		}
	case bytecode.OpMapGet:
		var v []bytecode.Value
		if v, e = m.popN(2); e != nil {
			break
		}
		var mp *bytecode.Map
		if mp, e = m.mapOf(v[0]); e != nil {
			break
		}
		var k bytecode.MapKey
		if k, e = m.key(v[1]); e == nil {
			if i, ok := mp.Find(k); ok {
				m.push(some(mp.Vals[i]))
			} else {
				m.push(none())
			}
		}
	case bytecode.OpMapKeys:
		var v bytecode.Value
		if v, e = m.pop(); e == nil {
			var mp *bytecode.Map
			if mp, e = m.mapOf(v); e == nil {
				m.push(bytecode.ArrayValue(append([]bytecode.Value(nil), mp.Keys...)))
			}
		}
	case bytecode.OpIsVariant:
		var v bytecode.Value
		if v, e = m.pop(); e == nil {
			ev := v.AsEnum()
			if v.Kind != bytecode.KindEnum || ev == nil {
				e = m.invalid("IS_VARIANT of a %s value", v.Kind)
				break
			}
			m.push(bytecode.BoolValue(ev.Enum == in.A && ev.Variant == in.B))
		}
	case bytecode.OpEnumField:
		var v bytecode.Value
		if v, e = m.pop(); e == nil {
			ev := v.AsEnum()
			if v.Kind != bytecode.KindEnum || ev == nil || in.A >= len(ev.Fields) {
				e = m.invalid("ENUM_FIELD %d of a %s value", in.A, v.Kind)
				break
			}
			f := ev.Fields[in.A]
			bytecode.MarkShared(f)
			m.push(f)
		}
	case bytecode.OpEqValue, bytecode.OpNeValue:
		var v []bytecode.Value
		if v, e = m.popN(2); e == nil {
			eq := bytecode.Equal(v[0], v[1])
			m.push(bytecode.BoolValue(eq == (in.Op == bytecode.OpEqValue)))
		}
	case bytecode.OpMakeStruct:
		var fields []bytecode.Value
		if fields, e = m.popN(len(m.mod.Structs[in.A].Fields)); e == nil {
			for _, v := range fields {
				bytecode.MarkShared(v)
			}
			m.push(bytecode.StructOf(in.A, fields))
		}
	case bytecode.OpGetField:
		var v bytecode.Value
		if v, e = m.pop(); e == nil {
			var s *bytecode.StructValue
			var i int
			if s, i, e = m.fieldOf(v, bytecode.IntValue(int64(in.A))); e == nil {
				f := s.Fields[i]
				bytecode.MarkShared(f)
				m.push(f)
			}
		}
	case bytecode.OpTakePath:
		// The receiver of a mut self call (ADR-0014): every node on the
		// path is made exclusively the variable's, so the callee can change
		// the value in place; PUT_PATH stores the result back.
		var keys []bytecode.Value
		if keys, e = m.popN(in.B); e == nil {
			var ref *bytecode.Value
			if ref, e = m.writeRef(fr, in.A, keys); e == nil {
				m.push(*ref)
			}
		}
	case bytecode.OpPutPath:
		var vals []bytecode.Value
		if vals, e = m.popN(in.B + 1); e != nil {
			break
		}
		if in.B == 0 {
			fr.locals[in.A] = vals[0]
			break
		}
		var ref *bytecode.Value
		if ref, e = m.writeRef(fr, in.A, vals[:in.B-1]); e != nil {
			break
		}
		last, value := vals[in.B-1], vals[in.B]
		switch ref.Kind {
		case bytecode.KindArray:
			var i int
			if i, e = m.position(ref.AsArray(), last); e == nil {
				ref.AsArray().Elems[i] = value
			}
		case bytecode.KindMap:
			var k bytecode.MapKey
			if k, e = m.key(last); e == nil {
				ref.AsMap().Set(last, k, value)
			}
		case bytecode.KindStruct:
			var s *bytecode.StructValue
			var i int
			if s, i, e = m.fieldOf(*ref, last); e == nil {
				s.Fields[i] = value
			}
		default:
			e = m.invalid("cannot index a %s value", ref.Kind)
		}
	case bytecode.OpAssert:
		var v bytecode.Value
		if v, e = m.pop(); e == nil {
			switch {
			case v.Kind != bytecode.KindBool:
				e = m.invalid("ASSERT of a %s value", v.Kind)
			case !v.Bool:
				e = m.fail(CodeAssertion, "assertion failed")
			default:
				m.push(bytecode.UnitValue())
			}
		}
	case bytecode.OpAssertEq:
		var v []bytecode.Value
		if v, e = m.popN(2); e == nil {
			if bytecode.Equal(v[0], v[1]) {
				m.push(bytecode.UnitValue())
			} else {
				e = m.fail(CodeAssertion, "assertEqual failed: %s != %s", quoteValue(m.mod, v[0]), quoteValue(m.mod, v[1]))
			}
		}
	case bytecode.OpPrintValue:
		var v bytecode.Value
		if v, e = m.pop(); e == nil {
			if _, werr := m.out.Write([]byte(FormatIn(m.mod, v) + "\n")); werr != nil {
				return true, werr
			}
			m.push(bytecode.UnitValue())
		}
	case bytecode.OpToStringValue:
		var v bytecode.Value
		if v, e = m.pop(); e == nil {
			m.push(bytecode.StringValue(FormatIn(m.mod, v)))
		}
	default:
		return false, nil
	}
	if e != nil {
		return true, e
	}
	return true, nil
}

// FormatIn returns the program-visible text of any value (ADR-0013). A
// string on its own is its contents; inside a collection or an enum it
// is quoted, so ["a", "b"] reads unambiguously.
func FormatIn(mod *bytecode.Module, v bytecode.Value) string {
	if v.Kind == bytecode.KindString {
		return v.Str
	}
	var b strings.Builder
	writeValue(&b, mod, v)
	return b.String()
}

func quoteValue(mod *bytecode.Module, v bytecode.Value) string {
	var b strings.Builder
	writeValue(&b, mod, v)
	return b.String()
}

func writeValue(b *strings.Builder, mod *bytecode.Module, v bytecode.Value) {
	enums := mod.Enums
	switch v.Kind {
	case bytecode.KindString:
		writeQuoted(b, v.Str)
	case bytecode.KindArray:
		a := v.AsArray()
		b.WriteByte('[')
		for i, e := range a.Elems {
			if i > 0 {
				b.WriteString(", ")
			}
			writeValue(b, mod, e)
		}
		b.WriteByte(']')
	case bytecode.KindMap:
		mp := v.AsMap()
		if mp.Len() == 0 {
			b.WriteString("[:]")
			return
		}
		b.WriteByte('[')
		for i := range mp.Keys {
			if i > 0 {
				b.WriteString(", ")
			}
			writeValue(b, mod, mp.Keys[i])
			b.WriteString(": ")
			writeValue(b, mod, mp.Vals[i])
		}
		b.WriteByte(']')
	case bytecode.KindEnum:
		ev := v.AsEnum()
		if ev.Enum >= 0 && ev.Enum < len(enums) && ev.Variant >= 0 && ev.Variant < len(enums[ev.Enum].Variants) {
			b.WriteString(enums[ev.Enum].Name + "." + enums[ev.Enum].Variants[ev.Variant].Name)
		} else {
			b.WriteString("enum(" + strconv.Itoa(ev.Enum) + "." + strconv.Itoa(ev.Variant) + ")")
		}
		if len(ev.Fields) > 0 {
			b.WriteByte('(')
			for i, f := range ev.Fields {
				if i > 0 {
					b.WriteString(", ")
				}
				writeValue(b, mod, f)
			}
			b.WriteByte(')')
		}
	case bytecode.KindStruct:
		// Point { x: 1, y: 2 } (ADR-0014)
		s := v.AsStruct()
		var info *bytecode.StructInfo
		if s.Struct >= 0 && s.Struct < len(mod.Structs) && len(mod.Structs[s.Struct].Fields) == len(s.Fields) {
			info = &mod.Structs[s.Struct]
			b.WriteString(info.Name)
		} else {
			b.WriteString("struct(" + strconv.Itoa(s.Struct) + ")")
		}
		if len(s.Fields) == 0 {
			b.WriteString(" {}")
			return
		}
		b.WriteString(" { ")
		for i, f := range s.Fields {
			if i > 0 {
				b.WriteString(", ")
			}
			if info != nil {
				b.WriteString(info.Fields[i] + ": ")
			}
			writeValue(b, mod, f)
		}
		b.WriteString(" }")
	default:
		b.WriteString(Format(v))
	}
}

// writeQuoted writes s as a Tread string literal.
func writeQuoted(b *strings.Builder, s string) {
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '$':
			if i+1 < len(s) && s[i+1] == '{' {
				b.WriteString(`\$`)
			} else {
				b.WriteByte('$')
			}
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
}
