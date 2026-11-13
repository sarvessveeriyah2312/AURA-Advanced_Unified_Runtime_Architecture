package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// client drives the server over in-memory pipes, as an editor would.
type client struct {
	t      *testing.T
	in     *io.PipeWriter
	out    *bufio.Reader
	nextID int
	done   chan error
	// notes collects notifications received while waiting for responses.
	notes []map[string]any
}

func start(t *testing.T) *client {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := &client{t: t, in: inW, out: bufio.NewReader(outR), done: make(chan error, 1)}
	go func() {
		c.done <- Serve(inR, outW, "test")
		outW.Close()
	}()
	t.Cleanup(func() { inW.Close() })
	return c
}

func (c *client) send(v map[string]any) {
	v["jsonrpc"] = "2.0"
	body, _ := json.Marshal(v)
	fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

func (c *client) readMsg() map[string]any {
	c.t.Helper()
	ch := make(chan map[string]any, 1)
	go func() {
		conn := &conn{r: c.out}
		body, err := conn.read()
		if err != nil {
			ch <- nil
			return
		}
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		ch <- m
	}()
	select {
	case m := <-ch:
		if m == nil {
			c.t.Fatal("server closed the stream")
		}
		return m
	case <-time.After(10 * time.Second):
		c.t.Fatal("timed out waiting for the server")
	}
	return nil
}

// request sends a request and returns its response, keeping
// notifications that arrive first.
func (c *client) request(method string, params any) map[string]any {
	c.t.Helper()
	c.nextID++
	id := c.nextID
	c.send(map[string]any{"id": id, "method": method, "params": params})
	for {
		m := c.readMsg()
		if _, isResp := m["id"]; isResp && m["method"] == nil {
			if int(m["id"].(float64)) != id {
				c.t.Fatalf("response for id %v, want %d", m["id"], id)
			}
			return m
		}
		c.notes = append(c.notes, m)
	}
}

func (c *client) notify(method string, params any) {
	c.send(map[string]any{"method": method, "params": params})
}

// diagnostics waits for the next publishDiagnostics for uri.
func (c *client) diagnostics(uri string) []any {
	c.t.Helper()
	for i, n := range c.notes {
		if p, ok := n["params"].(map[string]any); ok && n["method"] == "textDocument/publishDiagnostics" && p["uri"] == uri {
			c.notes = append(c.notes[:i], c.notes[i+1:]...)
			return p["diagnostics"].([]any)
		}
	}
	for {
		m := c.readMsg()
		if p, ok := m["params"].(map[string]any); ok && m["method"] == "textDocument/publishDiagnostics" && p["uri"] == uri {
			return p["diagnostics"].([]any)
		}
		c.notes = append(c.notes, m)
	}
}

func pos(line, char int) map[string]any { return map[string]any{"line": line, "character": char} }

const mainSrc = `import geometry

struct Box<T> {
    value: T
}

fn twice(n: Int) -> Int {
    return n * 2
}

fn main() -> Unit {
    let mut p = geometry.Point { x: 1, y: 2 }
    let b = Box { value: "é😀s" }
    print(twice(p.x))
    p.
}
`

const libSrc = `pub struct Point {
    pub x: Int
    pub y: Int
}

pub fn origin() -> Point {
    return Point { x: 0, y: 0 }
}
`

func TestSession(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.tr")
	if err := os.WriteFile(filepath.Join(dir, "geometry.tr"), []byte(libSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	mainURI := pathToURI(mainPath)
	c := start(t)

	// Requests before initialize are refused.
	if r := c.request("textDocument/hover", map[string]any{}); r["error"] == nil {
		t.Errorf("hover before initialize: %v", r)
	}
	init := c.request("initialize", map[string]any{"capabilities": map[string]any{}})
	caps := init["result"].(map[string]any)["capabilities"].(map[string]any)
	if caps["hoverProvider"] != true || caps["documentFormattingProvider"] != true {
		t.Errorf("capabilities = %v", caps)
	}
	c.notify("initialized", map[string]any{})

	// The incomplete `p.` is a syntax error.
	c.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": mainURI, "languageId": "tread", "version": 1, "text": mainSrc}})
	diags := c.diagnostics(mainURI)
	if len(diags) != 1 || !strings.Contains(fmt.Sprint(diags[0]), "TR-P") {
		t.Fatalf("diagnostics = %v", diags)
	}

	// Fix it: the text parses and checks.
	fixed := strings.Replace(mainSrc, "    p.\n", "    p.x = 3\n", 1)
	c.notify("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": mainURI, "version": 2},
		"contentChanges": []any{map[string]any{"text": fixed}}})
	if diags := c.diagnostics(mainURI); len(diags) != 0 {
		t.Fatalf("diagnostics after the fix = %v", diags)
	}

	hoverAt := func(line, char int) string {
		r := c.request("textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": mainURI}, "position": pos(line, char)})
		res, _ := r["result"].(map[string]any)
		if res == nil {
			return ""
		}
		return res["contents"].(map[string]any)["value"].(string)
	}
	for _, tc := range []struct {
		line, char int
		want       string
	}{
		{13, 11, "fn twice(n: Int) -> Int"},
		{13, 17, "let mut p: geometry.Point"},
		{13, 19, "field x: Int"},
		{11, 26, "struct Point {\n    pub x: Int"},
		{0, 9, "import geometry"},
		{12, 8, "let b: Box<String>"},
		{7, 11, "n: Int"},
		{13, 5, "fn print"},
	} {
		if got := hoverAt(tc.line, tc.char); !strings.Contains(got, tc.want) {
			t.Errorf("hover %d:%d = %q, want %q", tc.line, tc.char, got, tc.want)
		}
	}

	defAt := func(line, char int) (string, float64) {
		r := c.request("textDocument/definition", map[string]any{"textDocument": map[string]any{"uri": mainURI}, "position": pos(line, char)})
		res, _ := r["result"].(map[string]any)
		if res == nil {
			return "", -1
		}
		start := res["range"].(map[string]any)["start"].(map[string]any)
		return res["uri"].(string), start["line"].(float64)
	}
	if uri, line := defAt(13, 11); uri != mainURI || line != 6 {
		t.Errorf("definition of twice = %s:%v", uri, line)
	}
	if uri, line := defAt(13, 19); !strings.HasSuffix(uri, "geometry.tr") || line != 1 {
		t.Errorf("definition of field x = %s:%v", uri, line)
	}
	if uri, line := defAt(0, 9); !strings.HasSuffix(uri, "geometry.tr") || line != 0 {
		t.Errorf("definition of the import = %s:%v", uri, line)
	}
	if uri, line := defAt(12, 13); uri != mainURI || line != 2 {
		t.Errorf("definition of Box = %s:%v", uri, line)
	}

	// Completion after "p." uses the last analysis that parsed.
	c.notify("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": mainURI, "version": 3},
		"contentChanges": []any{map[string]any{"text": mainSrc}}})
	c.diagnostics(mainURI)
	r := c.request("textDocument/completion", map[string]any{"textDocument": map[string]any{"uri": mainURI}, "position": pos(14, 6)})
	labels := fmt.Sprint(r["result"])
	if !strings.Contains(labels, "label:x") || !strings.Contains(labels, "label:y") {
		t.Errorf("completion after p. = %s", labels)
	}
	r = c.request("textDocument/completion", map[string]any{"textDocument": map[string]any{"uri": mainURI}, "position": pos(12, 4)})
	labels = fmt.Sprint(r["result"])
	for _, want := range []string{"label:twice", "label:geometry", "label:let", "label:Box", "label:p]"} {
		if !strings.Contains(labels, want) {
			t.Errorf("completion is missing %q: %s", want, labels)
		}
	}

	// Formatting returns one edit for the whole document.
	messy := "fn main()->Unit{print(1)}\n"
	c.notify("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": mainURI, "version": 4},
		"contentChanges": []any{map[string]any{"text": messy}}})
	c.diagnostics(mainURI)
	r = c.request("textDocument/formatting", map[string]any{"textDocument": map[string]any{"uri": mainURI}, "options": map[string]any{}})
	edits := r["result"].([]any)
	if len(edits) != 1 || edits[0].(map[string]any)["newText"] != "fn main() -> Unit { print(1) }\n" {
		t.Errorf("formatting = %v", edits)
	}

	r = c.request("textDocument/documentSymbol", map[string]any{"textDocument": map[string]any{"uri": mainURI}})
	if fmt.Sprint(r["result"]) == "[]" || !strings.Contains(fmt.Sprint(r["result"]), "name:main") {
		t.Errorf("symbols = %v", r["result"])
	}

	if r := c.request("workspace/unknown", nil); r["error"] == nil {
		t.Error("unknown method accepted")
	}
	c.request("shutdown", nil)
	c.notify("exit", nil)
	select {
	case err := <-c.done:
		if err != nil {
			t.Errorf("Serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("server did not exit")
	}
}

// An imported file with an error gets diagnostics under its own URI, and
// an open, unsaved buffer of it is used instead of the file on disk.
func TestImportedDiagnosticsAndBuffers(t *testing.T) {
	dir := t.TempDir()
	mainURI := pathToURI(filepath.Join(dir, "main.tr"))
	libPath := filepath.Join(dir, "lib.tr")
	libURI := pathToURI(libPath)
	_ = os.WriteFile(libPath, []byte("pub fn f() -> Int {\n    return \"x\"\n}\n"), 0o644)
	c := start(t)
	c.request("initialize", map[string]any{})
	c.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": mainURI, "version": 1,
		"text": "import lib\nfn main() -> Unit {\n    print(lib.f())\n}\n"}})
	if d := c.diagnostics(libURI); len(d) != 1 || !strings.Contains(fmt.Sprint(d), "TR-T001") {
		t.Errorf("lib diagnostics = %v", d)
	}
	if d := c.diagnostics(mainURI); len(d) != 0 {
		t.Errorf("main diagnostics = %v", d)
	}
	// Open the library with the error fixed but unsaved.
	c.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": libURI, "version": 1,
		"text": "pub fn f() -> Int {\n    return 1\n}\n"}})
	if d := c.diagnostics(libURI); len(d) != 0 {
		t.Errorf("lib diagnostics with the buffer = %v", d)
	}
}

func TestPositions(t *testing.T) {
	text := []byte("ab\r\né😀x\ny")
	for _, tc := range []struct {
		off int
		p   position
	}{{0, position{0, 0}}, {2, position{0, 2}}, {4, position{1, 0}}, {6, position{1, 1}}, {10, position{1, 3}}, {11, position{1, 4}}, {12, position{2, 0}}, {13, position{2, 1}}} {
		if got := positionOf(text, tc.off); got != tc.p {
			t.Errorf("positionOf(%d) = %v, want %v", tc.off, got, tc.p)
		}
		if got := offsetOf(text, tc.p); got != tc.off {
			t.Errorf("offsetOf(%v) = %d, want %d", tc.p, got, tc.off)
		}
	}
	if p := filepath.Join("a", "b c.tr"); uriToPath(pathToURI(p)) != p && !strings.Contains(uriToPath(pathToURI(p)), "b c.tr") {
		t.Errorf("URI round trip of %q = %q", p, uriToPath(pathToURI(p)))
	}
}
