package bytecode

import "fmt"

// Heap values (ADR-0013). Arrays and maps have value semantics in Tread,
// implemented with copy-on-write: Shared is set whenever a value may have
// a second owner, and the VM clones a shared value before changing it.
// Enum values are immutable and never change after construction.

// Array is the payload of a KindArray value.
type Array struct {
	Elems  []Value
	Shared bool
}

// Map is the payload of a KindMap value: an insertion-ordered map. Keys
// are Int, String or Bool values.
type Map struct {
	Keys   []Value
	Vals   []Value
	index  map[MapKey]int
	Shared bool
}

// EnumValue is the payload of a KindEnum value.
type EnumValue struct {
	Enum    int // index in Module.Enums
	Variant int
	Fields  []Value
}

// StructValue is the payload of a KindStruct value (ADR-0014). Like
// arrays and maps, structs have value semantics with copy-on-write.
type StructValue struct {
	Struct int // index in Module.Structs
	Fields []Value
	Shared bool
}

// StructOf returns a struct value.
func StructOf(s int, fields []Value) Value {
	return Value{Kind: KindStruct, Ref: &StructValue{Struct: s, Fields: fields}}
}

// AsStruct returns the payload of a struct value.
func (v Value) AsStruct() *StructValue { s, _ := v.Ref.(*StructValue); return s }

// MapKey is the comparable form of a map key.
type MapKey struct {
	Kind Kind
	Int  int64
	Str  string
	Bool bool
}

// KeyOf returns the map key for v, and whether v can be a key.
func KeyOf(v Value) (MapKey, bool) {
	switch v.Kind {
	case KindInt:
		return MapKey{Kind: KindInt, Int: v.Int}, true
	case KindString:
		return MapKey{Kind: KindString, Str: v.Str}, true
	case KindBool:
		return MapKey{Kind: KindBool, Bool: v.Bool}, true
	}
	return MapKey{}, false
}

// ArrayValue returns an array value holding elems (not copied).
func ArrayValue(elems []Value) Value {
	return Value{Kind: KindArray, Ref: &Array{Elems: elems}}
}

// NewMap returns an empty map value.
func NewMap() Value { return Value{Kind: KindMap, Ref: &Map{index: map[MapKey]int{}}} }

// EnumOf returns an enum value.
func EnumOf(enum, variant int, fields []Value) Value {
	return Value{Kind: KindEnum, Ref: &EnumValue{Enum: enum, Variant: variant, Fields: fields}}
}

// AsArray, AsMap and AsEnum return the payload of a heap value.
func (v Value) AsArray() *Array    { a, _ := v.Ref.(*Array); return a }
func (v Value) AsMap() *Map        { m, _ := v.Ref.(*Map); return m }
func (v Value) AsEnum() *EnumValue { e, _ := v.Ref.(*EnumValue); return e }

// MarkShared records that v may now have a second owner. It does
// nothing for other values.
func MarkShared(v Value) {
	switch r := v.Ref.(type) {
	case *Array:
		r.Shared = true
	case *Map:
		r.Shared = true
	case *StructValue:
		r.Shared = true
	}
}

// Len returns the number of entries.
func (m *Map) Len() int { return len(m.Keys) }

// Find returns the position of key k.
func (m *Map) Find(k MapKey) (int, bool) {
	i, ok := m.index[k]
	return i, ok
}

// Set inserts or replaces the value of key k (whose Value is key).
func (m *Map) Set(key Value, k MapKey, val Value) {
	if i, ok := m.index[k]; ok {
		m.Vals[i] = val
		return
	}
	if m.index == nil {
		m.index = map[MapKey]int{}
	}
	m.index[k] = len(m.Keys)
	m.Keys = append(m.Keys, key)
	m.Vals = append(m.Vals, val)
}

// Delete removes key k, keeping the insertion order of the others. It
// returns the removed value.
func (m *Map) Delete(k MapKey) (Value, bool) {
	i, ok := m.index[k]
	if !ok {
		return Value{}, false
	}
	val := m.Vals[i]
	delete(m.index, k)
	m.Keys = append(m.Keys[:i], m.Keys[i+1:]...)
	m.Vals = append(m.Vals[:i], m.Vals[i+1:]...)
	for j := i; j < len(m.Keys); j++ {
		kk, _ := KeyOf(m.Keys[j])
		m.index[kk] = j
	}
	return val, true
}

// Clone returns an unshared copy of a shared array or map. The elements
// are now owned by both copies, so they are marked shared.
func Clone(v Value) Value {
	switch r := v.Ref.(type) {
	case *Array:
		elems := make([]Value, len(r.Elems), len(r.Elems)+1)
		copy(elems, r.Elems)
		for _, e := range elems {
			MarkShared(e)
		}
		return Value{Kind: KindArray, Ref: &Array{Elems: elems}}
	case *Map:
		c := &Map{
			Keys:  append([]Value(nil), r.Keys...),
			Vals:  append([]Value(nil), r.Vals...),
			index: make(map[MapKey]int, len(r.index)),
		}
		for k, i := range r.index {
			c.index[k] = i
		}
		for _, e := range c.Vals {
			MarkShared(e)
		}
		return Value{Kind: KindMap, Ref: c}
	case *StructValue:
		fields := append([]Value(nil), r.Fields...)
		for _, f := range fields {
			MarkShared(f)
		}
		return Value{Kind: KindStruct, Ref: &StructValue{Struct: r.Struct, Fields: fields}}
	}
	return v
}

// Equal reports structural equality (ADR-0013). Floats compare as
// IEEE-754 numbers, so NaN is unequal to everything. Maps are equal when
// they hold the same entries, in any order.
func Equal(a, b Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case KindUnit:
		return true
	case KindInt:
		return a.Int == b.Int
	case KindFloat:
		return a.Float == b.Float
	case KindBool:
		return a.Bool == b.Bool
	case KindString:
		return a.Str == b.Str
	case KindArray:
		x, y := a.AsArray(), b.AsArray()
		if len(x.Elems) != len(y.Elems) {
			return false
		}
		for i := range x.Elems {
			if !Equal(x.Elems[i], y.Elems[i]) {
				return false
			}
		}
		return true
	case KindMap:
		x, y := a.AsMap(), b.AsMap()
		if x.Len() != y.Len() {
			return false
		}
		for i, key := range x.Keys {
			k, _ := KeyOf(key)
			j, ok := y.Find(k)
			if !ok || !Equal(x.Vals[i], y.Vals[j]) {
				return false
			}
		}
		return true
	case KindEnum:
		x, y := a.AsEnum(), b.AsEnum()
		if x.Enum != y.Enum || x.Variant != y.Variant || len(x.Fields) != len(y.Fields) {
			return false
		}
		for i := range x.Fields {
			if !Equal(x.Fields[i], y.Fields[i]) {
				return false
			}
		}
		return true
	case KindStruct:
		x, y := a.AsStruct(), b.AsStruct()
		if x.Struct != y.Struct || len(x.Fields) != len(y.Fields) {
			return false
		}
		for i := range x.Fields {
			if !Equal(x.Fields[i], y.Fields[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// StructInfo describes a struct for display and validation (ADR-0014).
type StructInfo struct {
	Name   string
	Fields []string
}

// EnumInfo describes an enum for display and validation.
type EnumInfo struct {
	Name     string
	Variants []VariantInfo
}

// VariantInfo is one variant and its number of fields.
type VariantInfo struct {
	Name   string
	Fields int
}

func (e EnumInfo) String() string { return fmt.Sprintf("%s%v", e.Name, e.Variants) }
