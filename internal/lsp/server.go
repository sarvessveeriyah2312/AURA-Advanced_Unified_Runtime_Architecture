package lsp

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/sarvessveeriyah2312/Tread/internal/checker"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/format"
	"github.com/sarvessveeriyah2312/Tread/internal/pipeline"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// Serve runs the language server until the client sends `exit` or closes
// the input. version is reported to the client.
func Serve(in io.Reader, out io.Writer, version string) error {
	s := &server{
		conn:      newConn(in, out),
		version:   version,
		docs:      map[string]*document{},
		published: map[string][]string{},
		lastGood:  map[string]*analysis{},
	}
	return s.loop()
}

type document struct {
	uri  string
	path string
	text []byte
}

type server struct {
	conn        *conn
	version     string
	initialized bool
	shutdown    bool
	docs        map[string]*document // by URI
	// published lists, for each open document, the URIs it published
	// diagnostics to, so they can be cleared.
	published map[string][]string
	// lastGood is the last analysis of each document that parsed, used
	// for completion while the text does not parse.
	lastGood map[string]*analysis
}

func (s *server) loop() error {
	for {
		body, err := s.conn.read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var m message
		if err := json.Unmarshal(body, &m); err != nil {
			if err := s.conn.write(failure{JSONRPC: "2.0", Error: &responseError{Code: codeParseError, Message: "invalid JSON"}}); err != nil {
				return err
			}
			continue
		}
		if m.Method == "exit" {
			return nil
		}
		if m.Method == "" {
			continue // a response to a request we never send
		}
		result, rerr := s.handle(&m)
		if m.ID == nil {
			continue // notifications get no response
		}
		if rerr != nil {
			err = s.conn.write(failure{JSONRPC: "2.0", ID: m.ID, Error: rerr})
		} else {
			err = s.conn.write(success{JSONRPC: "2.0", ID: m.ID, Result: result})
		}
		if err != nil {
			return err
		}
	}
}

func invalidParams(err error) *responseError {
	return &responseError{Code: codeInvalidParams, Message: err.Error()}
}

func (s *server) handle(m *message) (any, *responseError) {
	if !s.initialized && m.Method != "initialize" {
		if m.ID == nil {
			return nil, nil
		}
		return nil, &responseError{Code: codeServerNotInit, Message: "the server is not initialized"}
	}
	switch m.Method {
	case "initialize":
		s.initialized = true
		return map[string]any{
			"capabilities": map[string]any{
				"textDocumentSync": map[string]any{
					"openClose": true,
					"change":    1, // full text
					"save":      map[string]any{"includeText": false},
				},
				"hoverProvider":              true,
				"definitionProvider":         true,
				"documentFormattingProvider": true,
				"documentSymbolProvider":     true,
				"completionProvider":         map[string]any{"triggerCharacters": []string{"."}},
			},
			"serverInfo": map[string]any{"name": "tread", "version": s.version},
		}, nil
	case "initialized", "$/cancelRequest", "$/setTrace", "workspace/didChangeConfiguration":
		return nil, nil
	case "shutdown":
		s.shutdown = true
		return nil, nil
	case "textDocument/didOpen":
		var p struct {
			TextDocument textDocumentItem `json:"textDocument"`
		}
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return nil, invalidParams(err)
		}
		s.docs[p.TextDocument.URI] = &document{uri: p.TextDocument.URI, path: uriToPath(p.TextDocument.URI), text: []byte(p.TextDocument.Text)}
		s.refresh()
		return nil, nil
	case "textDocument/didChange":
		var p struct {
			TextDocument   textDocumentIdentifier `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return nil, invalidParams(err)
		}
		d := s.docs[p.TextDocument.URI]
		if d == nil || len(p.ContentChanges) == 0 {
			return nil, nil
		}
		d.text = []byte(p.ContentChanges[len(p.ContentChanges)-1].Text)
		s.refresh()
		return nil, nil
	case "textDocument/didSave":
		s.refresh() // imported files may have changed on disk
		return nil, nil
	case "textDocument/didClose":
		var p struct {
			TextDocument textDocumentIdentifier `json:"textDocument"`
		}
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return nil, invalidParams(err)
		}
		uri := p.TextDocument.URI
		delete(s.docs, uri)
		delete(s.lastGood, uri)
		for _, u := range s.published[uri] {
			s.publish(u, nil)
		}
		delete(s.published, uri)
		s.refresh()
		return nil, nil
	case "textDocument/hover":
		return s.withPosition(m, s.hover)
	case "textDocument/definition":
		return s.withPosition(m, s.definition)
	case "textDocument/completion":
		return s.withPosition(m, s.completion)
	case "textDocument/formatting":
		var p struct {
			TextDocument textDocumentIdentifier `json:"textDocument"`
		}
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return s.formatting(p.TextDocument.URI), nil
	case "textDocument/documentSymbol":
		var p struct {
			TextDocument textDocumentIdentifier `json:"textDocument"`
		}
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return s.symbols(p.TextDocument.URI), nil
	}
	if m.ID == nil {
		return nil, nil
	}
	return nil, &responseError{Code: codeMethodNotFound, Message: "method not supported: " + m.Method}
}

func (s *server) withPosition(m *message, f func(d *document, off int) any) (any, *responseError) {
	var p positionParams
	if err := json.Unmarshal(m.Params, &p); err != nil {
		return nil, invalidParams(err)
	}
	d := s.docs[p.TextDocument.URI]
	if d == nil {
		return nil, nil
	}
	return f(d, offsetOf(d.text, p.Position)), nil
}

// ---------------------------------------------------------------------------
// Analysis and diagnostics

// analysis is one check of a document and the files it imports.
type analysis struct {
	doc     *document
	checked *checker.CheckedProgram // nil when the text does not parse
	files   source.Set
	diags   []diagnostic.Diagnostic
	uris    map[source.FileID]string
	texts   map[source.FileID][]byte
}

func (a *analysis) uri(id source.FileID) string { return a.uris[id] }

// analyze checks a document. Open documents take precedence over files
// on disk, also for imports.
func (s *server) analyze(d *document) *analysis {
	byPath := map[string]*document{}
	for _, doc := range s.docs {
		byPath[filepath.Clean(doc.path)] = doc
	}
	dir := filepath.Dir(d.path)
	load := func(rel string) (string, []byte, error) {
		name := filepath.Join(dir, filepath.FromSlash(rel))
		if doc := byPath[filepath.Clean(name)]; doc != nil {
			return name, doc.text, nil
		}
		content, err := os.ReadFile(name)
		var pe *fs.PathError
		if errors.As(err, &pe) {
			err = pe.Err
		}
		return name, content, err
	}
	root := source.NewFile(1, d.path, d.text)
	checked, files, diags := pipeline.Compiler{Load: load}.Inspect(root)
	a := &analysis{doc: d, checked: checked, files: files, diags: diags,
		uris: map[source.FileID]string{}, texts: map[source.FileID][]byte{}}
	for _, f := range files {
		a.texts[f.ID()] = f.Content()
		if f.ID() == root.ID() {
			a.uris[f.ID()] = d.uri
		} else if doc := byPath[filepath.Clean(f.Name())]; doc != nil {
			a.uris[f.ID()] = doc.uri
		} else {
			a.uris[f.ID()] = pathToURI(f.Name())
		}
	}
	if checked != nil {
		s.lastGood[d.uri] = a
	}
	return a
}

// refresh checks every open document and publishes its diagnostics.
func (s *server) refresh() {
	uris := make([]string, 0, len(s.docs))
	for u := range s.docs {
		uris = append(uris, u)
	}
	sort.Strings(uris)
	for _, u := range uris {
		a := s.analyze(s.docs[u])
		byURI := map[string][]lspDiagnostic{a.doc.uri: nil}
		for _, d := range a.diags {
			fileURI := a.uri(d.Span.File)
			if fileURI == "" {
				fileURI = a.doc.uri
			}
			byURI[fileURI] = append(byURI[fileURI], s.toLSP(a, d))
		}
		// An imported file's diagnostics also belong to its own analysis
		// when it is open; publish them here only when it is not.
		var sent []string
		for fileURI, list := range byURI {
			if fileURI != a.doc.uri && s.docs[fileURI] != nil {
				continue
			}
			s.publish(fileURI, list)
			sent = append(sent, fileURI)
		}
		for _, old := range s.published[u] {
			if _, still := byURI[old]; !still && s.docs[old] == nil {
				s.publish(old, nil)
			}
		}
		s.published[u] = sent
	}
}

func (s *server) publish(uri string, diags []lspDiagnostic) {
	if diags == nil {
		diags = []lspDiagnostic{}
	}
	_ = s.conn.write(notification{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics",
		Params: map[string]any{"uri": uri, "diagnostics": diags}})
}

func (s *server) toLSP(a *analysis, d diagnostic.Diagnostic) lspDiagnostic {
	text := a.texts[d.Span.File]
	if text == nil {
		text = a.doc.text
	}
	msg := d.Message
	if d.Help != "" {
		msg += "\nhelp: " + d.Help
	}
	out := lspDiagnostic{Range: rangeOf(text, d.Span), Severity: 1, Code: string(d.Code), Source: "tread", Message: msg}
	for _, r := range d.Related {
		u := a.uri(r.Span.File)
		rt := a.texts[r.Span.File]
		if u == "" || rt == nil {
			continue
		}
		out.RelatedInformation = append(out.RelatedInformation, diagnosticInfo{
			Location: location{URI: u, Range: rangeOf(rt, r.Span)}, Message: r.Message})
	}
	return out
}

// ---------------------------------------------------------------------------
// Formatting

func (s *server) formatting(uri string) []textEdit {
	d := s.docs[uri]
	if d == nil {
		return nil
	}
	out, diags, err := format.File(source.NewFile(1, d.path, d.text))
	if len(diags) > 0 || err != nil || string(out) == string(d.text) {
		return []textEdit{}
	}
	end := positionOf(d.text, len(d.text))
	return []textEdit{{Range: lspRange{End: end}, NewText: string(out)}}
}
