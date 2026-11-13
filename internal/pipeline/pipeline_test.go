package pipeline

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/sarvessveeriyah2312/Tread/internal/diagnostic"
	"github.com/sarvessveeriyah2312/Tread/internal/source"
	"github.com/sarvessveeriyah2312/Tread/internal/vm"
)

// memory returns a loader over in-memory files and counts the loads.
func memory(files map[string]string, loads map[string]int) Loader {
	return func(rel string) (string, []byte, error) {
		loads[rel]++
		text, ok := files[rel]
		if !ok {
			return rel, nil, fs.ErrNotExist
		}
		return rel, []byte(text), nil
	}
}

func analyze(t *testing.T, root string, files map[string]string) (source.Set, []diagnostic.Diagnostic, map[string]int) {
	t.Helper()
	loads := map[string]int{}
	c := Compiler{Load: memory(files, loads)}
	set, diags := c.Check(source.NewFile(1, "main.tr", []byte(root)))
	return set, diags, loads
}

func TestLoadsEachFileOnce(t *testing.T) {
	files := map[string]string{
		"a.tr":       "import b.c\npub fn a() -> Int {\n    return c.c()\n}\n",
		"b/c.tr":     "pub fn c() -> Int {\n    return 3\n}\n",
		"diamond.tr": "import b.c\nimport a\npub fn d() -> Int {\n    return a.a() + c.c()\n}\n",
	}
	set, diags, loads := analyze(t, "import a\nimport diamond\nfn main() -> Unit {\n    print(a.a() + diamond.d())\n}\n", files)
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %+v", diags)
	}
	if len(set) != 4 || set[0].Name() != "main.tr" {
		t.Errorf("files = %d", len(set))
	}
	for rel, n := range loads {
		if n != 1 {
			t.Errorf("%s loaded %d times", rel, n)
		}
	}
	for i, f := range set {
		if f.ID() != source.FileID(i+1) {
			t.Errorf("file %s has ID %d, want %d", f.Name(), f.ID(), i+1)
		}
	}
}

func TestImportErrors(t *testing.T) {
	tests := []struct {
		name, root string
		files      map[string]string
		want       []string
	}{
		{"missing", "import gone\nfn main() -> Unit { }\n", nil,
			[]string{"TR-T036 cannot import 'gone': gone.tr: no such file"}},
		{"self cycle", "import a\nfn main() -> Unit { }\n", map[string]string{"a.tr": "import a\n"},
			[]string{"TR-T035 import cycle: a imports a"}},
		{"root cycle", "import a\nfn main() -> Unit { }\n", map[string]string{"a.tr": "import main\n", "main.tr": "import a\n"},
			[]string{"TR-T035 import cycle: a imports main imports a"}},
		{"syntax in import", "import a\nfn main() -> Unit { }\n", map[string]string{"a.tr": "fn (\n"},
			[]string{"TR-P002 expected function name, found '('"}},
		{"module name", "import a\nfn main() -> Unit { }\n", map[string]string{"a.tr": "module b\n"},
			[]string{"TR-T033 this file is imported as 'a', but declares 'module b'"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, diags, _ := analyze(t, tc.root, tc.files)
			var got []string
			for _, d := range diags {
				got = append(got, string(d.Code)+" "+d.Message)
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}

func TestLoaderErrorText(t *testing.T) {
	c := Compiler{Load: func(rel string) (string, []byte, error) { return rel, nil, errors.New("disk on fire") }}
	_, diags := c.Check(source.NewFile(1, "main.tr", []byte("import x\nfn main() -> Unit { }\n")))
	if len(diags) != 1 || !strings.HasSuffix(diags[0].Message, "x.tr: disk on fire") {
		t.Errorf("diags = %+v", diags)
	}
}

func TestCompileRunsAcrossFiles(t *testing.T) {
	loads := map[string]int{}
	c := Compiler{Load: memory(map[string]string{"util.tr": "pub fn twice(x: Int) -> Int {\n    return x * 2\n}\n"}, loads)}
	m, set, diags := c.Compile(source.NewFile(1, "main.tr", []byte("import util\nfn main() -> Unit {\n    print(util.twice(21))\n}\n")))
	if len(diags) > 0 || len(set) != 2 {
		t.Fatalf("diags %+v files %d", diags, len(set))
	}
	var out strings.Builder
	if err := vm.Execute(m, &out); err != nil || out.String() != "42\n" {
		t.Errorf("run: %q %v", out.String(), err)
	}
	if m.Functions[0].Name != "util.twice" || m.Functions[0].Spans[0].File != 2 {
		t.Errorf("imported function = %s in file %d", m.Functions[0].Name, m.Functions[0].Spans[0].File)
	}
}
