// Package pipeline runs the compiler phases in order (docs/04 §4.1): it
// loads a program's files (ADR-0014), then lexes, parses, checks and
// compiles them.
package pipeline

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarvessveeriyah2312/Tread/internal/ast"
	"github.com/sarvessveeriyah2312/Tread/internal/bytecode"
	"github.com/sarvessveeriyah2312/Tread/internal/checker"
	"github.com/sarvessveeriyah2312/Tread/internal/compiler"
	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/lexer"
	"github.com/sarvessveeriyah2312/Tread/internal/parser"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
)

// Loader reads an imported file. rel is its path relative to the root
// file's directory, with '/' separators, such as "shapes/circle.tr". It
// returns the file's display name and content.
type Loader func(rel string) (name string, content []byte, err error)

// Compiler implements cli.Compiler with the real compiler phases.
type Compiler struct {
	// Load reads imported files. When nil, they are read from the file
	// system, next to the root file.
	Load Loader
}

// Check analyzes a program without compiling it. It returns every loaded
// file, root first, for rendering diagnostics.
func (c Compiler) Check(file *source.File) (source.Set, []diagnostic.Diagnostic) {
	_, files, diags := c.Analyze(file)
	return files, diags
}

// Compile analyzes and compiles a program.
func (c Compiler) Compile(file *source.File) (*bytecode.Module, source.Set, []diagnostic.Diagnostic) {
	checked, files, diags := c.Analyze(file)
	if len(diags) > 0 {
		return nil, files, diags
	}
	m, diags := compiler.Compile(checked)
	return m, files, diags
}

// Analyze loads, lexes, parses and checks the program whose root file is
// file. Syntax and loading errors stop it before checking.
func (c Compiler) Analyze(file *source.File) (*checker.CheckedProgram, source.Set, []diagnostic.Diagnostic) {
	l := &loader{load: c.Load, root: file, index: map[string]int{"": 0}}
	if l.load == nil {
		l.load = diskLoader(file.Name())
	}
	l.add(file, "")
	l.visit(0)
	if len(l.diags) > 0 {
		sortDiags(l.diags)
		return nil, l.files, l.diags
	}
	checked, diags := checker.CheckModules(l.mods)
	if len(diags) > 0 {
		return nil, l.files, diags
	}
	return checked, l.files, nil
}

// Analyze checks a single-file program with the default loader.
func Analyze(file *source.File) (*checker.CheckedProgram, []diagnostic.Diagnostic) {
	checked, _, diags := Compiler{}.Analyze(file)
	return checked, diags
}

// Build checks and compiles a program with the default loader.
func Build(file *source.File) (*bytecode.Module, []diagnostic.Diagnostic) {
	m, _, diags := Compiler{}.Compile(file)
	return m, diags
}

// diskLoader reads imports relative to the directory of the root file.
func diskLoader(rootName string) Loader {
	dir := filepath.Dir(rootName)
	return func(rel string) (string, []byte, error) {
		name := filepath.Join(dir, filepath.FromSlash(rel))
		content, err := os.ReadFile(name)
		var pe *fs.PathError
		if errors.As(err, &pe) {
			err = pe.Err
		}
		return name, content, err
	}
}

// loader builds the module graph: each imported file is loaded once, by
// depth-first search, which also finds import cycles (ADR-0014).
type loader struct {
	load  Loader
	root  *source.File
	files source.Set
	mods  []checker.SourceModule
	index map[string]int // import path -> module
	state []int          // 0 unvisited, 1 on the current path, 2 done
	stack []string       // import paths on the current path
	diags []diagnostic.Diagnostic
}

func (l *loader) add(file *source.File, path string) int {
	toks, lexDiags := lexer.Lex(file)
	prog, parseDiags := parser.Parse(toks)
	l.diags = append(append(l.diags, lexDiags...), parseDiags...)
	l.files = append(l.files, file)
	l.mods = append(l.mods, checker.SourceModule{Path: path, Program: prog, Imports: map[*ast.ImportDecl]int{}})
	l.state = append(l.state, 0)
	l.index[path] = len(l.mods) - 1
	return len(l.mods) - 1
}

func (l *loader) visit(i int) {
	l.state[i] = 1
	l.stack = append(l.stack, l.mods[i].Path)
	for _, d := range l.mods[i].Program.Imports {
		path := d.PathString()
		j, seen := l.index[path]
		if !seen {
			name, content, err := l.load(strings.ReplaceAll(path, ".", "/") + ".tr")
			if err != nil {
				reason := err.Error()
				if errors.Is(err, fs.ErrNotExist) {
					reason = "no such file" // the same text on every OS
				}
				l.diags = append(l.diags, diagnostic.Errorf(diagnostic.ImportNotFound, d.Loc,
					"cannot import '%s': %s: %s", path, name, reason).
					WithHelp("imports are found relative to the directory of the file being run"))
				continue
			}
			file := source.NewFile(source.FileID(len(l.files)+1), name, content)
			j = l.add(file, path)
		}
		l.mods[i].Imports[d] = j
		switch l.state[j] {
		case 0:
			l.visit(j)
		case 1:
			cycle := l.stack[indexOf(l.stack, l.mods[j].Path):]
			names := make([]string, 0, len(cycle)+1)
			for _, p := range cycle {
				names = append(names, l.display(p))
			}
			names = append(names, l.display(l.mods[j].Path))
			l.diags = append(l.diags, diagnostic.Errorf(diagnostic.ImportCycle, d.Loc,
				"import cycle: %s", strings.Join(names, " imports ")).
				WithHelp("move the declarations both files need into a third file that both import"))
			delete(l.mods[i].Imports, d)
		}
	}
	l.stack = l.stack[:len(l.stack)-1]
	l.state[i] = 2
}

// display names a module in a cycle message: the root file by its file
// name, others by import path.
func (l *loader) display(path string) string {
	if path == "" {
		return filepath.Base(l.root.Name())
	}
	return path
}

func indexOf(xs []string, x string) int {
	for i, s := range xs {
		if s == x {
			return i
		}
	}
	return 0
}

func sortDiags(diags []diagnostic.Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i].Span, diags[j].Span
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Start < b.Start
	})
}
