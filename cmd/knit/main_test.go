package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withnic/knit/internal/graphschema"
)

func runCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(context.Background(), args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRun_Usage(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantOut    string
		wantErrOut string
	}{
		{name: "no args", args: nil, wantCode: exitUsage, wantErrOut: "Usage:"},
		{name: "help", args: []string{"help"}, wantCode: exitOK, wantOut: "knit gen [flags]"},
		{name: "version", args: []string{"version"}, wantCode: exitOK, wantOut: "knit dev\n"},
		{name: "unknown command", args: []string{"build"}, wantCode: exitUsage, wantErrOut: `unknown command "build"`},
		{name: "gen -h", args: []string{"gen", "-h"}, wantCode: exitOK, wantErrOut: "-graph-out"},
		{name: "gen bad flag", args: []string{"gen", "-nope"}, wantCode: exitUsage, wantErrOut: "flag provided but not defined"},
		{name: "gen bad mode", args: []string{"gen", "-mode=dagger"}, wantCode: exitUsage, wantErrOut: `unknown mode "dagger"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, out, errOut := runCLI(t, tt.args...)
			if code != tt.wantCode || !strings.Contains(out, tt.wantOut) || !strings.Contains(errOut, tt.wantErrOut) {
				t.Errorf("run(%v) = %d\nstdout: %s\nstderr: %s\nwant %d, stdout ~ %q, stderr ~ %q",
					tt.args, code, out, errOut, tt.wantCode, tt.wantOut, tt.wantErrOut)
			}
		})
	}
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// genModule returns a small application whose injector is declared in the
// given mode's dialect, with a local stub of the marker package.
func genModule(t *testing.T, mode string) map[string]string {
	t.Helper()
	files := map[string]string{
		"store/store.go": `package store

type Store struct{ closed bool }

func NewStore() (*Store, func(), error) {
	s := &Store{}
	return s, func() { s.closed = true }, nil
}

func (s *Store) Get() string { return "world" }
`,
		"svc/svc.go": `package svc

import "example.com/pipe/store"

type Svc struct{ st *store.Store }

func NewSvc(st *store.Store) *Svc { return &Svc{st: st} }

func (s *Svc) Hello() string { return "hello " + s.st.Get() }
`,
		"di/di_test.go": `package di

import "testing"

func TestInitializeSvc(t *testing.T) {
	s, cleanup, err := InitializeSvc()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if got := s.Hello(); got != "hello world" {
		t.Fatalf("Hello() = %q", got)
	}
}
`,
	}
	var markerPath, markerSrc string
	switch mode {
	case "knit":
		markerPath = "github.com/withnic/knit"
		src, err := os.ReadFile(filepath.Join("..", "..", "knit.go"))
		if err != nil {
			t.Fatal(err)
		}
		markerSrc = string(src)
	case "wire":
		markerPath = "github.com/google/wire"
		markerSrc = "package wire\n\nfunc Build(...interface{}) string { return \"\" }\n\n" +
			"type ProviderSet struct{}\n\nfunc NewSet(...interface{}) ProviderSet { return ProviderSet{} }\n"
	}
	files["go.mod"] = "module example.com/pipe\n\ngo 1.27\n\nrequire " + markerPath + " v0.0.0\n\nreplace " + markerPath + " => ./third_party/marker\n"
	files["third_party/marker/go.mod"] = "module " + markerPath + "\n\ngo 1.27\n"
	files["third_party/marker/marker.go"] = markerSrc
	files["di/"+mode+".go"] = `//go:build ` + mode + `inject

package di

import (
	"example.com/pipe/store"
	"example.com/pipe/svc"
	"` + markerPath + `"
)

func InitializeSvc() (*svc.Svc, func(), error) {
	panic(` + mode + `.Build(store.NewStore, svc.NewSvc))
}
`
	return files
}

// TestGen_EndToEnd runs knit gen the way a user would, in both modes:
// -dry-run prints without writing, gen writes the generated file and the
// graph, a second run is a no-op, and the generated injector compiles and
// works.
func TestGen_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go list / go test")
	}
	tests := []struct {
		mode    string
		args    []string // mode flags
		outFile string
	}{
		{mode: "knit", args: nil, outFile: "knit_gen.go"},
		{mode: "wire", args: []string{"-mode=wire"}, outFile: "wire_gen.go"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			dir := t.TempDir()
			for rel, content := range genModule(t, tt.mode) {
				write(t, dir, rel, content)
			}
			t.Setenv("GOWORK", "off")
			t.Setenv("GOFLAGS", "-mod=mod")
			t.Setenv("GOPROXY", "off")
			t.Chdir(dir)
			gen := func(extra ...string) []string { return append(append([]string{"gen"}, tt.args...), extra...) }
			outRel := "di/" + tt.outFile

			code, out, errOut := runCLI(t, gen("-dry-run", "./...")...)
			if code != exitOK || !strings.Contains(out, "// ==> "+outRel+" <==\n// Code generated by knit. DO NOT EDIT.\n\n//go:build !"+tt.mode+"inject\n") {
				t.Fatalf("gen -dry-run = %d\nstdout: %s\nstderr: %s", code, out, errOut)
			}
			if _, err := os.Stat(filepath.Join(dir, outRel)); !os.IsNotExist(err) {
				t.Fatalf("gen -dry-run wrote %s (stat err = %v)", outRel, err)
			}

			code, _, errOut = runCLI(t, gen()...)
			if code != exitOK || !strings.Contains(errOut, "wrote "+outRel) || !strings.Contains(errOut, "wrote .knit/di-graph.json") {
				t.Fatalf("gen = %d\nstderr: %s", code, errOut)
			}
			g, err := graphschema.ReadFile(filepath.Join(dir, graphschema.DefaultPath))
			if err != nil {
				t.Fatalf("reading graph: %v", err)
			}
			if len(g.Injectors) != 1 || g.Injectors[0].Name != "InitializeSvc" || len(g.Injectors[0].Dependencies) != 2 {
				t.Errorf("graph = %+v, want InitializeSvc with 2 dependencies", g.Injectors)
			}

			first, err := os.ReadFile(filepath.Join(dir, outRel))
			if err != nil {
				t.Fatal(err)
			}
			if code, _, errOut = runCLI(t, gen("-graph-out", "")...); code != exitOK {
				t.Fatalf("second gen = %d\nstderr: %s", code, errOut)
			}
			second, err := os.ReadFile(filepath.Join(dir, outRel))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first, second) {
				t.Errorf("regenerated %s differs:\n%s\n---\n%s", outRel, first, second)
			}

			cmd := exec.Command("go", "test", "-count=1", "./di")
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("go test ./di: %v\n%s", err, out)
			}
		})
	}
}

func TestGen_ErrorExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go list")
	}
	dir := t.TempDir()
	write(t, dir, "go.mod", "module example.com/empty\n\ngo 1.27\n")
	write(t, dir, "a/a.go", "package a\n")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOPROXY", "off")
	t.Chdir(dir)
	code, _, errOut := runCLI(t, "gen")
	if code != exitError || !strings.Contains(errOut, "no injectors found") || !strings.Contains(errOut, "-mode=wire") {
		t.Errorf("gen = %d, stderr: %s", code, errOut)
	}
}
