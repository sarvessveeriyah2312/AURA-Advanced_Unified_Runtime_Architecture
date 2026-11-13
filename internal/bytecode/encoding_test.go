package bytecode

import (
	"encoding/binary"
	"hash/crc32"
	"math"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// sampleSource is the text the sample module's spans point into.
const sampleSource = "fn add(a: Int, b: Int) -> Int { return a + b }\nfn main() -> Unit { print(add(1, 2)) }\n"

// tbcModule exercises every constant kind and a call.
func tbcModule() *Module {
	sp := source.Span{File: 7, Start: 1, End: 3}
	spans := func(n int) []source.Span {
		s := make([]source.Span, n)
		for i := range s {
			s[i] = sp
		}
		return s
	}
	add := Function{Name: "add", NumParams: 2, NumLocals: 2, Result: KindInt, Code: []Instr{
		{OpLoadLocal, 0, 0}, {OpLoadLocal, 1, 0}, {OpAddInt, 0, 0}, {OpReturn, 0, 0},
	}}
	// The if-statement shape: both paths reach pc 15 with an empty stack.
	main := Function{Name: "main", NumLocals: 1, Result: KindUnit, Code: []Instr{
		{OpConst, 0, 0}, {OpConst, 1, 0}, {OpCall, 0, 0}, {OpPrintInt, 0, 0}, {OpPop, 0, 0},
		{OpConst, 2, 0}, {OpPrintFloat, 0, 0}, {OpPop, 0, 0},
		{OpConst, 3, 0}, {OpStoreLocal, 0, 0},
		{OpLoadLocal, 0, 0}, {OpJumpIfFalse, 15, 0}, {OpConst, 4, 0}, {OpPrintString, 0, 0}, {OpPop, 0, 0},
		{OpUnit, 0, 0}, {OpReturn, 0, 0},
	}}
	add.Spans, main.Spans = spans(len(add.Code)), spans(len(main.Code))
	return &Module{
		Constants: []Value{IntValue(-5), IntValue(math.MaxInt64), FloatValue(math.Copysign(0, -1)), BoolValue(true), StringValue("héllo")},
		Functions: []Function{add, main},
		Main:      1,
	}
}

func sampleFile() *File {
	return &File{Producer: "tread test", SourceName: "sample.tr", Source: []byte(sampleSource), Module: tbcModule()}
}

func mustEncode(t *testing.T, f *File) []byte {
	t.Helper()
	data, err := Encode(f)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return data
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	f := sampleFile()
	if err := f.Module.Verify(); err != nil {
		t.Fatalf("sample module does not verify: %v", err)
	}
	data := mustEncode(t, f)
	if string(data[:4]) != Magic || binary.LittleEndian.Uint16(data[4:]) != FormatVersion {
		t.Fatalf("bad header % x", data[:8])
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Producer != f.Producer || got.SourceName != f.SourceName || string(got.Source) != sampleSource {
		t.Errorf("metadata = %q %q %q", got.Producer, got.SourceName, got.Source)
	}
	if got.Module.Disassemble() != f.Module.Disassemble() {
		t.Errorf("disassembly differs:\n%s\nwant:\n%s", got.Module.Disassemble(), f.Module.Disassemble())
	}
	if !SameFloat(got.Module.Constants[2].Float, math.Copysign(0, -1)) {
		t.Error("-0.0 constant lost its sign")
	}
	for _, fn := range got.Module.Functions {
		for _, s := range fn.Spans {
			if s != (source.Span{File: SourceFileID, Start: 1, End: 3}) {
				t.Fatalf("span = %+v", s)
			}
		}
	}
	// Encoding is deterministic.
	if again := mustEncode(t, got); string(again) != string(data) {
		t.Error("re-encoding a decoded file changed its bytes")
	}
}

func TestEncodeRejects(t *testing.T) {
	f := sampleFile()
	f.Module.Main = 9
	if _, err := Encode(f); err == nil {
		t.Error("encoded an invalid module")
	}
	f = sampleFile()
	f.Source = []byte("x") // spans end at 3
	if _, err := Encode(f); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Errorf("span outside source: err = %v", err)
	}
	if _, err := Encode(&File{}); err == nil {
		t.Error("encoded a file without a module")
	}
}

// reseal recomputes the checksum after a test edits the body.
func reseal(data []byte) []byte {
	body := data[:len(data)-4]
	return binary.LittleEndian.AppendUint32(append([]byte(nil), body...), crc32.ChecksumIEEE(body))
}

func TestDecodeRejects(t *testing.T) {
	good := mustEncode(t, sampleFile())
	edit := func(fn func(b []byte) []byte) []byte {
		return fn(append([]byte(nil), good...))
	}
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", nil, "not a Tread bytecode file"},
		{"bad magic", edit(func(b []byte) []byte { b[0] = 'X'; return b }), "not a Tread bytecode file"},
		{"source file", []byte("fn main() -> Unit {}\n"), "not a Tread bytecode file"},
		{"header only", good[:8], "truncated"},
		{"future version", edit(func(b []byte) []byte { b[4] = 3; return b }), "unsupported bytecode format version 3: this runtime supports version 2"},
		{"0.4 format", edit(func(b []byte) []byte { b[4] = 1; return b }), "unsupported bytecode format version 1: this runtime supports version 2; rebuild"},
		{"version zero", edit(func(b []byte) []byte { b[4] = 0; return b }), "unsupported bytecode format version 0"},
		{"flags", edit(func(b []byte) []byte { b[6] = 1; return b }), "unsupported bytecode flags"},
		{"checksum", edit(func(b []byte) []byte { b[len(b)-1] ^= 0xFF; return b }), "checksum mismatch"},
		{"flipped body byte", edit(func(b []byte) []byte { b[20] ^= 0x01; return b }), "checksum mismatch"},
		{"trailing bytes", edit(func(b []byte) []byte { return reseal(append(b[:len(b)-4], 0, 0, 0, 0, 0)) }), "trailing bytes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(tc.data)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
			if _, ok := err.(*FormatError); err != nil && !ok {
				t.Errorf("err is %T, want *FormatError", err)
			}
		})
	}
}

// TestDecodeTruncatedEverywhere cuts the file at every length, with and
// without a recomputed checksum: each must be rejected without a panic.
func TestDecodeTruncatedEverywhere(t *testing.T) {
	good := mustEncode(t, sampleFile())
	for n := 0; n < len(good); n++ {
		if _, err := Decode(good[:n]); err == nil {
			t.Fatalf("accepted a file truncated to %d bytes", n)
		}
		if n >= 8 && n < len(good)-4 { // len(good)-4 is the whole body
			if _, err := Decode(reseal(append(good[:n:n], 0, 0, 0, 0))); err == nil {
				t.Fatalf("accepted a resealed file truncated to %d bytes", n)
			}
		}
	}
}

// encodeRaw encodes a module without Encode's validation, so tests can
// produce files whose content is invalid but well formed.
func encodeRaw(t *testing.T, src string, m *Module) []byte {
	t.Helper()
	valid := &Module{Functions: []Function{{Name: "main", NumLocals: 0, Code: []Instr{{OpUnit, 0, 0}, {OpReturn, 0, 0}}, Spans: make([]source.Span, 2)}}}
	data := mustEncode(t, &File{Source: []byte(src), Module: valid})
	// Rebuild the body from scratch with the raw module.
	b := append([]byte(nil), data[:headerLen]...)
	b = appendString(b, "")
	b = appendString(b, "")
	b = appendBytes(b, []byte(src))
	b = binary.AppendUvarint(b, uint64(len(m.Constants)))
	for _, c := range m.Constants {
		b = append(b, byte(c.Kind))
		switch c.Kind {
		case KindInt:
			b = binary.AppendVarint(b, c.Int)
		case KindString:
			b = appendString(b, c.Str)
		case KindBool:
			b = append(b, byte(c.Int)) // raw byte, for invalid Bool tests
		}
	}
	b = binary.AppendUvarint(b, uint64(len(m.Functions)))
	for _, fn := range m.Functions {
		b = appendString(b, fn.Name)
		b = binary.AppendUvarint(b, uint64(fn.NumParams))
		b = binary.AppendUvarint(b, uint64(fn.NumLocals))
		b = append(b, byte(fn.Result))
		b = binary.AppendUvarint(b, uint64(len(fn.Code)))
		for pc, in := range fn.Code {
			b = append(b, byte(in.Op))
			b = binary.AppendUvarint(b, uint64(in.A))
			b = binary.AppendUvarint(b, uint64(in.B))
			var s source.Span
			if pc < len(fn.Spans) {
				s = fn.Spans[pc]
			}
			b = binary.AppendUvarint(b, uint64(s.Start))
			b = binary.AppendUvarint(b, uint64(s.End))
		}
	}
	b = binary.AppendUvarint(b, uint64(m.Main))
	b = binary.AppendUvarint(b, 0) // no enums
	return binary.LittleEndian.AppendUint32(b, crc32.ChecksumIEEE(b))
}

func fn(name string, params, locals int, code ...Instr) Function {
	return Function{Name: name, NumParams: params, NumLocals: locals, Code: code}
}

func TestDecodeRejectsInvalidContent(t *testing.T) {
	ret := []Instr{{OpUnit, 0, 0}, {OpReturn, 0, 0}}
	tests := []struct {
		name string
		src  string
		m    *Module
		want string
	}{
		{"constant index", "", &Module{Functions: []Function{fn("main", 0, 0, Instr{OpConst, 3, 0}, Instr{OpReturn, 0, 0})}}, "CONST operand 3 out of range"},
		{"unknown opcode", "", &Module{Functions: []Function{fn("main", 0, 0, Instr{Opcode(250), 0, 0}, Instr{OpReturn, 0, 0})}}, "unknown opcode 250"},
		{"jump target", "", &Module{Functions: []Function{fn("main", 0, 0, Instr{OpJump, 9, 0}, ret[1])}}, "JUMP operand 9 out of range"},
		{"slot", "", &Module{Functions: []Function{fn("main", 0, 0, Instr{OpLoadLocal, 0, 0}, ret[1])}}, "LOAD_LOCAL operand 0 out of range"},
		{"call index", "", &Module{Functions: []Function{fn("main", 0, 0, Instr{OpCall, 1, 0}, ret[1])}}, "CALL operand 1 out of range"},
		{"main index", "", &Module{Functions: []Function{fn("main", 0, 0, ret...)}, Main: 4}, "main index 4 out of range"},
		{"main params", "", &Module{Functions: []Function{fn("main", 1, 1, ret...)}}, "main must have no parameters"},
		{"stack underflow", "", &Module{Functions: []Function{fn("main", 0, 0, Instr{OpAddInt, 0, 0}, ret[0], ret[1])}}, "ADD_INT needs 2 stack values, has 0"},
		{"return on empty stack", "", &Module{Functions: []Function{fn("main", 0, 0, Instr{OpReturn, 0, 0})}}, "RETURN needs 1 stack values, has 0"},
		{"call arguments", "", &Module{Functions: []Function{fn("main", 0, 0, Instr{OpCall, 1, 0}, Instr{OpReturn, 0, 0}), fn("f", 2, 2, ret...)}}, "CALL needs 2 stack values, has 0"},
		{"falls off the end", "", &Module{Functions: []Function{fn("main", 0, 0, Instr{OpUnit, 0, 0}, Instr{OpPop, 0, 0})}}, "past the end of the code"},
		{"inconsistent depth", "", &Module{Functions: []Function{fn("main", 0, 0,
			Instr{OpConst, 0, 0}, Instr{OpJumpIfFalse, 3, 0}, Instr{OpUnit, 0, 0}, Instr{OpUnit, 0, 0}, Instr{OpReturn, 0, 0})}, Constants: []Value{BoolValue(true)}}, "reached with stack depth"},
		{"span outside source", "ab", &Module{Functions: []Function{{Name: "main", Code: ret, Spans: []source.Span{{Start: 0, End: 3}, {}}}}}, "outside the 2-byte source"},
		{"unknown kind", "", &Module{Constants: []Value{{Kind: 9}}, Functions: []Function{fn("main", 0, 0, ret...)}}, "unknown value kind 9"},
		{"bad bool", "", &Module{Constants: []Value{{Kind: KindBool, Int: 2}}, Functions: []Function{fn("main", 0, 0, ret...)}}, "invalid Bool constant 0"},
		{"invalid UTF-8", "", &Module{Constants: []Value{StringValue("\xff")}, Functions: []Function{fn("main", 0, 0, ret...)}}, "not valid UTF-8"},
		{"empty body", "", &Module{Functions: []Function{fn("main", 0, 0)}}, "empty body"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(encodeRaw(t, tc.src, tc.m))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestDecodeRejectsHugeCounts(t *testing.T) {
	b := []byte(Magic)
	b = binary.LittleEndian.AppendUint16(b, FormatVersion)
	b = binary.LittleEndian.AppendUint16(b, 0)
	b = appendString(b, "")
	b = appendString(b, "")
	b = appendBytes(b, nil)
	b = binary.AppendUvarint(b, math.MaxInt32) // constant count
	b = binary.LittleEndian.AppendUint32(b, crc32.ChecksumIEEE(b))
	if _, err := Decode(b); err == nil || !strings.Contains(err.Error(), "exceeds the remaining") {
		t.Errorf("err = %v", err)
	}
}

func TestVerifyAcceptsLoops(t *testing.T) {
	// A backward jump at a consistent depth (as a future while loop would
	// compile) verifies.
	m := &Module{Constants: []Value{BoolValue(false)}, Functions: []Function{fn("main", 0, 0,
		Instr{OpConst, 0, 0}, Instr{OpJumpIfFalse, 3, 0}, Instr{OpJump, 0, 0}, Instr{OpUnit, 0, 0}, Instr{OpReturn, 0, 0})}}
	m.Functions[0].Spans = make([]source.Span, 5)
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
}

func FuzzDecode(f *testing.F) {
	f.Add(mustEncodeFuzz(sampleFile()))
	f.Add([]byte(Magic))
	f.Fuzz(func(t *testing.T, data []byte) {
		file, err := Decode(data)
		if err != nil {
			return
		}
		// Anything accepted must re-encode and decode to the same module.
		// (Bytes may differ: overlong varints decode but are not produced.)
		again, err := Encode(file)
		if err != nil {
			t.Fatalf("decoded file does not re-encode: %v", err)
		}
		back, err := Decode(again)
		if err != nil {
			t.Fatalf("re-encoded file does not decode: %v", err)
		}
		if back.Module.Disassemble() != file.Module.Disassemble() || string(back.Source) != string(file.Source) {
			t.Fatal("encode/decode changed an accepted file")
		}
	})
}

func mustEncodeFuzz(f *File) []byte {
	b, err := Encode(f)
	if err != nil {
		panic(err)
	}
	return b
}
