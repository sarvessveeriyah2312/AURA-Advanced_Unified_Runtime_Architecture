package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "App")
	if err := Create(dir, "App", "9.9.9"); err != nil {
		t.Fatal(err)
	}
	for rel, want := range Files("App", "9.9.9") {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Errorf("%s: %q, %v", rel, got, err)
		}
	}
	toml, _ := os.ReadFile(filepath.Join(dir, ConfigFile))
	for _, want := range []string{`name = "App"`, `entry = "src/main.tr"`, `tread = "9.9.9"`} {
		if !strings.Contains(string(toml), want) {
			t.Errorf("tread.toml lacks %s", want)
		}
	}
}

func TestCreateNeverOverwrites(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "App")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dir, "mine.txt")
	if err := os.WriteFile(keep, []byte("user data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Create(dir, "App", "1"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v", err)
	}
	if got, _ := os.ReadFile(keep); string(got) != "user data" {
		t.Error("existing file was changed")
	}
	if _, err := os.Stat(filepath.Join(dir, ConfigFile)); err == nil {
		t.Error("wrote into an existing directory")
	}
}

func TestNames(t *testing.T) {
	for _, ok := range []string{"a", "HelloWorld", "my-app", "app_2"} {
		if !validName.MatchString(ok) {
			t.Errorf("%q rejected", ok)
		}
	}
	for _, bad := range []string{"", "1a", "-a", "a b", "a/b", `a\b`, "..", "a.b", "${x}", "é"} {
		if err := Create(filepath.Join(t.TempDir(), "x"), bad, "1"); err == nil {
			t.Errorf("%q accepted", bad)
		} else if _, ok := err.(*NameError); !ok {
			t.Errorf("%q: %T", bad, err)
		}
	}
}
