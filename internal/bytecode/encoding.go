package bytecode

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
	"unicode/utf8"

	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// This file implements the Tread bytecode file format (.tbc), format
// version 1, specified in docs/05 §5.10 and ADR-0011. The encoding is
// written field by field; it never depends on Go type layouts.

const (
	// Extension is the file extension of Tread bytecode files.
	Extension = ".tbc"
	// Magic is the first four bytes of every bytecode file.
	Magic = "TRBC"
	// FormatVersion is the only bytecode format version this build reads
	// and writes. It changes whenever the encoding, the opcode numbering,
	// an opcode's semantics, or the value kinds change (ADR-0011).
	// Version 2 adds collections, enums and generics (ADR-0013); version
	// 3 adds structs, methods and multi-file programs (ADR-0014); version
	// 4 adds ASSERT and ASSERT_EQ (ADR-0015).
	// scripts/build-release.sh reads the next line; keep it bare.
	FormatVersion = 4

	headerLen   = len(Magic) + 2 + 2
	checksumLen = 4
	// maxCount bounds every count and operand, so decoded values always
	// fit in an int on every platform.
	maxCount = math.MaxInt32
)

// SourceFileID is the file ID of the root source file's spans in a
// decoded module; imported file i has ID SourceFileID+1+i (ADR-0014). The
// CLI gives the embedded sources these IDs when it renders diagnostics.
const SourceFileID source.FileID = 1

// File is the content of a bytecode file: a module, the source text it
// was compiled from (for runtime error excerpts), and who produced it.
type File struct {
	// Producer names the compiler, e.g. "tread 0.4.0". Informational:
	// it is never used to accept or reject a file.
	Producer   string
	SourceName string
	Source     []byte
	// Imports are the program's other source files (ADR-0014).
	Imports []SourceFile
	Module  *Module
}

// SourceFile is an embedded source file.
type SourceFile struct {
	Name   string
	Source []byte
}

// sourceLen returns the length of the embedded file whose spans have
// file ID id, and whether it exists.
func (f *File) sourceLen(i int) (int, bool) {
	switch {
	case i == 0:
		return len(f.Source), true
	case i-1 < len(f.Imports):
		return len(f.Imports[i-1].Source), true
	}
	return 0, false
}

// fileIndex returns the embedded file index of a function's spans.
func fileIndex(fn *Function) int {
	if len(fn.Spans) == 0 || fn.Spans[0].File <= SourceFileID {
		return 0
	}
	return int(fn.Spans[0].File - SourceFileID)
}

// FormatError reports a bytecode file that cannot be loaded. Its message
// is suitable for users (TR-R006).
type FormatError struct {
	Msg string
}

func (e *FormatError) Error() string { return e.Msg }

func formatErr(format string, args ...any) *FormatError {
	return &FormatError{Msg: fmt.Sprintf(format, args...)}
}

// Encode serializes f. The module must pass Validate; spans must lie
// within the embedded source.
func Encode(f *File) ([]byte, error) {
	if f == nil || f.Module == nil {
		return nil, fmt.Errorf("encode: no module")
	}
	m := f.Module
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	if err := checkSpans(m, f); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	b := make([]byte, 0, 256+len(f.Source))
	b = append(b, Magic...)
	b = binary.LittleEndian.AppendUint16(b, FormatVersion)
	b = binary.LittleEndian.AppendUint16(b, 0) // flags
	b = appendString(b, f.Producer)
	b = appendString(b, f.SourceName)
	b = appendBytes(b, f.Source)
	b = binary.AppendUvarint(b, uint64(len(f.Imports)))
	for _, s := range f.Imports {
		b = appendString(b, s.Name)
		b = appendBytes(b, s.Source)
	}

	b = binary.AppendUvarint(b, uint64(len(m.Constants)))
	for _, c := range m.Constants {
		b = append(b, byte(c.Kind))
		switch c.Kind {
		case KindInt:
			b = binary.AppendVarint(b, c.Int)
		case KindFloat:
			b = binary.LittleEndian.AppendUint64(b, math.Float64bits(c.Float))
		case KindBool:
			if c.Bool {
				b = append(b, 1)
			} else {
				b = append(b, 0)
			}
		case KindString:
			b = appendString(b, c.Str)
		case KindUnit:
		default:
			return nil, fmt.Errorf("encode: constant of unknown kind %d", c.Kind)
		}
	}

	b = binary.AppendUvarint(b, uint64(len(m.Functions)))
	for _, fn := range m.Functions {
		b = appendString(b, fn.Name)
		b = binary.AppendUvarint(b, uint64(fn.NumParams))
		b = binary.AppendUvarint(b, uint64(fn.NumLocals))
		b = append(b, byte(fn.Result))
		var flags byte
		if fn.MutSelf {
			flags = 1
		}
		b = append(b, flags)
		b = binary.AppendUvarint(b, uint64(fileIndex(&fn)))
		b = binary.AppendUvarint(b, uint64(len(fn.Code)))
		for pc, in := range fn.Code {
			b = append(b, byte(in.Op))
			b = binary.AppendUvarint(b, uint64(in.A)) // non-negative after Validate
			b = binary.AppendUvarint(b, uint64(in.B))
			b = binary.AppendUvarint(b, uint64(fn.Spans[pc].Start))
			b = binary.AppendUvarint(b, uint64(fn.Spans[pc].End))
		}
	}
	b = binary.AppendUvarint(b, uint64(m.Main))
	b = binary.AppendUvarint(b, uint64(len(m.Enums)))
	for _, e := range m.Enums {
		b = appendString(b, e.Name)
		b = binary.AppendUvarint(b, uint64(len(e.Variants)))
		for _, v := range e.Variants {
			b = appendString(b, v.Name)
			b = binary.AppendUvarint(b, uint64(v.Fields))
		}
	}
	b = binary.AppendUvarint(b, uint64(len(m.Structs)))
	for _, s := range m.Structs {
		b = appendString(b, s.Name)
		b = binary.AppendUvarint(b, uint64(len(s.Fields)))
		for _, name := range s.Fields {
			b = appendString(b, name)
		}
	}
	b = binary.LittleEndian.AppendUint32(b, crc32.ChecksumIEEE(b))
	return b, nil
}

func appendString(b []byte, s string) []byte {
	b = binary.AppendUvarint(b, uint64(len(s)))
	return append(b, s...)
}

func appendBytes(b []byte, p []byte) []byte {
	b = binary.AppendUvarint(b, uint64(len(p)))
	return append(b, p...)
}

// Decode parses and fully checks a bytecode file: header, checksum,
// structure, Validate and Verify. It never panics; every rejection is a
// *FormatError.
func Decode(data []byte) (*File, error) {
	if len(data) < len(Magic) || string(data[:len(Magic)]) != Magic {
		return nil, formatErr("not a Tread bytecode file (missing the %q header)", Magic)
	}
	if len(data) < headerLen+checksumLen {
		return nil, formatErr("truncated bytecode file (%d bytes)", len(data))
	}
	if v := binary.LittleEndian.Uint16(data[4:]); v != FormatVersion {
		return nil, formatErr("unsupported bytecode format version %d: this runtime supports version %d; rebuild the program with `tread build` from the same Tread release", v, FormatVersion)
	}
	if flags := binary.LittleEndian.Uint16(data[6:]); flags != 0 {
		return nil, formatErr("unsupported bytecode flags %#04x", flags)
	}
	body, sum := data[:len(data)-checksumLen], binary.LittleEndian.Uint32(data[len(data)-checksumLen:])
	if crc32.ChecksumIEEE(body) != sum {
		return nil, formatErr("bytecode file is corrupted or truncated (checksum mismatch)")
	}

	r := &reader{b: body, off: headerLen}
	f := &File{}
	f.Producer = r.str("producer")
	f.SourceName = r.str("source name")
	f.Source = r.bytes("source text")
	// An imported file needs at least 2 bytes: name and text lengths.
	nImports := r.count("source file", 2)
	if r.err == nil && nImports > 0 {
		f.Imports = make([]SourceFile, nImports)
	}
	for i := 0; i < nImports && r.err == nil; i++ {
		f.Imports[i] = SourceFile{Name: r.str("source name"), Source: r.bytes("source text")}
	}
	m := &Module{}
	f.Module = m

	nConst := r.count("constant", 1)
	if r.err == nil {
		m.Constants = make([]Value, nConst)
	}
	for i := 0; i < nConst && r.err == nil; i++ {
		m.Constants[i] = r.value(i)
	}

	// A function needs at least 7 bytes: name length, params, locals,
	// result, flags, file and instruction count.
	nFunc := r.count("function", 7)
	if r.err == nil {
		m.Functions = make([]Function, nFunc)
	}
	for i := 0; i < nFunc && r.err == nil; i++ {
		fn := &m.Functions[i]
		fn.Name = r.str("function name")
		fn.NumParams = r.int("parameter count")
		fn.NumLocals = r.int("local count")
		fn.Result = r.resultKind()
		switch flags := r.u8(); flags {
		case 0:
		case 1:
			fn.MutSelf = true
		default:
			r.fail("unknown function flags %#02x", flags)
		}
		fileIdx := r.int("source file index")
		if _, ok := f.sourceLen(fileIdx); r.err == nil && !ok {
			r.fail("function source file %d out of range", fileIdx)
		}
		// An instruction needs at least 5 bytes.
		n := r.count("instruction", 5)
		if r.err != nil {
			break
		}
		fn.Code = make([]Instr, n)
		fn.Spans = make([]source.Span, n)
		for pc := 0; pc < n && r.err == nil; pc++ {
			fn.Code[pc] = Instr{Op: Opcode(r.u8()), A: r.int("operand"), B: r.int("second operand")}
			start := r.int("span start")
			end := r.int("span end")
			fn.Spans[pc] = source.Span{File: SourceFileID + source.FileID(fileIdx), Start: start, End: end}
		}
	}
	m.Main = r.int("main index")
	// An enum needs at least 2 bytes: name length and variant count.
	nEnum := r.count("enum", 2)
	if r.err == nil {
		m.Enums = make([]EnumInfo, nEnum)
	}
	for i := 0; i < nEnum && r.err == nil; i++ {
		e := &m.Enums[i]
		e.Name = r.str("enum name")
		nv := r.count("variant", 2)
		if r.err != nil {
			break
		}
		e.Variants = make([]VariantInfo, nv)
		for j := 0; j < nv && r.err == nil; j++ {
			e.Variants[j] = VariantInfo{Name: r.str("variant name"), Fields: r.int("field count")}
		}
	}
	// A struct needs at least 2 bytes: name length and field count.
	nStruct := r.count("struct", 2)
	if r.err == nil && nStruct > 0 {
		m.Structs = make([]StructInfo, nStruct)
	}
	for i := 0; i < nStruct && r.err == nil; i++ {
		s := &m.Structs[i]
		s.Name = r.str("struct name")
		nf := r.count("field", 1)
		if r.err != nil {
			break
		}
		s.Fields = make([]string, nf)
		for j := 0; j < nf && r.err == nil; j++ {
			s.Fields[j] = r.str("field name")
		}
	}
	if r.err != nil {
		return nil, r.err
	}
	if r.off != len(body) {
		return nil, formatErr("bytecode file has %d unexpected trailing bytes", len(body)-r.off)
	}
	if err := checkSpans(m, f); err != nil {
		return nil, formatErr("invalid bytecode: %v", err)
	}
	if err := m.Verify(); err != nil {
		return nil, formatErr("invalid bytecode: %v", err)
	}
	return f, nil
}

// checkSpans requires every span to lie within its function's embedded
// source file, so the diagnostic renderer can trust decoded spans.
func checkSpans(m *Module, f *File) error {
	for _, fn := range m.Functions {
		idx := fileIndex(&fn)
		n, ok := f.sourceLen(idx)
		if !ok {
			return &ValidationError{Function: fn.Name, PC: -1, Msg: fmt.Sprintf("source file %d is not embedded", idx)}
		}
		for pc, s := range fn.Spans {
			if s.File != fn.Spans[0].File {
				return &ValidationError{Function: fn.Name, PC: pc, Msg: "spans from more than one source file"}
			}
			if s.Start < 0 || s.Start > s.End || s.End > n {
				return &ValidationError{Function: fn.Name, PC: pc, Msg: fmt.Sprintf("source span [%d,%d) outside the %d-byte source", s.Start, s.End, n)}
			}
		}
	}
	return nil
}

// reader decodes the body. After the first error every method returns a
// zero value, so callers check r.err once per loop.
type reader struct {
	b   []byte
	off int
	err *FormatError
}

func (r *reader) fail(format string, args ...any) {
	if r.err == nil {
		r.err = formatErr("malformed bytecode file at byte %d: %s", r.off, fmt.Sprintf(format, args...))
	}
}

func (r *reader) remaining() int { return len(r.b) - r.off }

func (r *reader) u8() byte {
	if r.err != nil {
		return 0
	}
	if r.remaining() < 1 {
		r.fail("unexpected end of data")
		return 0
	}
	v := r.b[r.off]
	r.off++
	return v
}

func (r *reader) uvarint(what string) uint64 {
	if r.err != nil {
		return 0
	}
	v, n := binary.Uvarint(r.b[r.off:])
	if n <= 0 {
		r.fail("invalid %s", what)
		return 0
	}
	r.off += n
	return v
}

// int reads a uvarint that must fit in [0, maxCount].
func (r *reader) int(what string) int {
	v := r.uvarint(what)
	if v > maxCount {
		r.fail("%s %d is too large", what, v)
		return 0
	}
	return int(v)
}

// count reads an element count and rejects counts that the remaining
// input cannot hold, so a malicious count cannot force a huge allocation.
func (r *reader) count(what string, minSize int) int {
	n := r.int(what + " count")
	if r.err == nil && n > r.remaining()/minSize {
		r.fail("%s count %d exceeds the remaining %d bytes", what, n, r.remaining())
		return 0
	}
	return n
}

func (r *reader) bytes(what string) []byte {
	n := r.int(what + " length")
	if r.err != nil {
		return nil
	}
	if n > r.remaining() {
		r.fail("%s length %d exceeds the remaining %d bytes", what, n, r.remaining())
		return nil
	}
	p := r.b[r.off : r.off+n : r.off+n]
	r.off += n
	return p
}

func (r *reader) str(what string) string {
	p := r.bytes(what)
	if r.err == nil && !utf8.Valid(p) {
		r.fail("%s is not valid UTF-8", what)
		return ""
	}
	return string(p)
}

// kind reads a constant's kind: constants are never heap values.
func (r *reader) kind() Kind {
	k := Kind(r.u8())
	if r.err == nil && k > KindString {
		r.fail("unknown value kind %d", k)
	}
	return k
}

// resultKind reads a function's result kind.
func (r *reader) resultKind() Kind {
	k := Kind(r.u8())
	if r.err == nil && k > KindAny {
		r.fail("unknown result kind %d", k)
	}
	return k
}

func (r *reader) value(i int) Value {
	k := r.kind()
	if r.err != nil {
		return Value{}
	}
	switch k {
	case KindInt:
		v, n := binary.Varint(r.b[r.off:])
		if n <= 0 {
			r.fail("invalid Int constant %d", i)
			return Value{}
		}
		r.off += n
		return IntValue(v)
	case KindFloat:
		if r.remaining() < 8 {
			r.fail("unexpected end of data in Float constant %d", i)
			return Value{}
		}
		v := math.Float64frombits(binary.LittleEndian.Uint64(r.b[r.off:]))
		r.off += 8
		return FloatValue(v)
	case KindBool:
		switch r.u8() {
		case 0:
			return BoolValue(false)
		case 1:
			return BoolValue(true)
		}
		r.fail("invalid Bool constant %d", i)
		return Value{}
	case KindString:
		return StringValue(r.str("String constant"))
	}
	return UnitValue()
}
