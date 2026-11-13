// Package lsp implements `tread lsp`, a Language Server Protocol 3.17
// server over stdin and stdout (ADR-0015). It has no dependencies beyond
// the standard library: JSON-RPC 2.0 messages framed with Content-Length
// headers.
package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
)

// message is an incoming JSON-RPC 2.0 message: a request (ID and
// Method), a notification (Method only), or a response to a request of
// ours, which this server never sends and ignores.
type message struct {
	ID     *json.RawMessage `json:"id,omitempty"`
	Method string           `json:"method,omitempty"`
	Params json.RawMessage  `json:"params,omitempty"`
}

// success, failure and notification are outgoing messages. A success
// always has a result, even null; a failure never has one.
type success struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  any              `json:"result"`
}

type failure struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Error   *responseError   `json:"error"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC and LSP error codes.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeServerNotInit  = -32002
)

// maxMessage bounds one message, so a bad header cannot exhaust memory.
const maxMessage = 64 << 20

// conn reads and writes framed messages.
type conn struct {
	r  *bufio.Reader
	w  io.Writer
	mu sync.Mutex
}

func newConn(r io.Reader, w io.Writer) *conn {
	return &conn{r: bufio.NewReader(r), w: w}
}

// read returns the next message body. io.EOF means the client closed the
// stream.
func (c *conn) read() ([]byte, error) {
	tp := textproto.NewReader(c.r)
	header, err := tp.ReadMIMEHeader()
	if err != nil {
		if errors.Is(err, io.EOF) && len(header) == 0 {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("reading header: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(header.Get("Content-Length")))
	if err != nil || n < 0 || n > maxMessage {
		return nil, fmt.Errorf("invalid Content-Length %q", header.Get("Content-Length"))
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	return body, nil
}

func (c *conn) write(m any) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.w.Write(body)
	return err
}

// ---------------------------------------------------------------------------
// LSP types (the subset this server uses).

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type location struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type textDocumentItem struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
	Text    string `json:"text"`
}

type positionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type diagnosticInfo struct {
	Location location `json:"location"`
	Message  string   `json:"message"`
}

type lspDiagnostic struct {
	Range              lspRange         `json:"range"`
	Severity           int              `json:"severity"`
	Code               string           `json:"code"`
	Source             string           `json:"source"`
	Message            string           `json:"message"`
	RelatedInformation []diagnosticInfo `json:"relatedInformation,omitempty"`
}

type textEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type hover struct {
	Contents markupContent `json:"contents"`
	Range    *lspRange     `json:"range,omitempty"`
}

type completionItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

type documentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          lspRange         `json:"range"`
	SelectionRange lspRange         `json:"selectionRange"`
	Children       []documentSymbol `json:"children,omitempty"`
}

// Completion item kinds and symbol kinds (LSP 3.17).
const (
	ciMethod   = 2
	ciFunction = 3
	ciField    = 5
	ciVariable = 6
	ciModule   = 9
	ciEnum     = 13
	ciKeyword  = 14
	ciEnumMem  = 20
	ciStruct   = 22

	skModule   = 2
	skMethod   = 6
	skField    = 8
	skEnum     = 10
	skFunction = 12
	skEnumMem  = 22
	skStruct   = 23
	skEvent    = 24
)
