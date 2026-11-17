package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
		{name: "tia bad flag", args: []string{"tia", "-nope"}, wantCode: exitUsage, wantErrOut: "flag provided but not defined"},
		{name: "tia extra args", args: []string{"tia", "./..."}, wantCode: exitUsage, wantErrOut: "unexpected arguments"},
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

func TestSplitList(t *testing.T) {
	got := splitList(" NewServer, ,Initialize* ,")
	if strings.Join(got, "|") != "NewServer|Initialize*" {
		t.Errorf("splitList() = %q", got)
	}
	if splitList("") != nil {
		t.Error("splitList(\"\") != nil")
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

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=knit", "GIT_AUTHOR_EMAIL=knit@example.invalid",
		"GIT_COMMITTER_NAME=knit", "GIT_COMMITTER_EMAIL=knit@example.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// pipelineModule is a small application using a local stub of
// github.com/google/wire.
var pipelineModule = map[string]string{
	"go.mod":                  "module example.com/pipe\n\ngo 1.27\n\nrequire github.com/google/wire v0.0.0\n\nreplace github.com/google/wire => ./third_party/wire\n",
	"third_party/wire/go.mod": "module github.com/google/wire\n\ngo 1.27\n",
	"third_party/wire/wire.go": "package wire\n\nfunc Build(...interface{}) string { return \"\" }\n\n" +
		"type ProviderSet struct{}\n\nfunc NewSet(...interface{}) ProviderSet { return ProviderSet{} }\n",
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
	"di/wire.go": `//go:build wireinject

package di

import (
	"example.com/pipe/store"
	"example.com/pipe/svc"
	"github.com/google/wire"
)

func InitializeSvc() (*svc.Svc, func(), error) {
	panic(wire.Build(store.NewStore, svc.NewSvc))
}
`,
	"di/di_integration_test.go": `package di

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
	"other/other.go":                  "package other\n\nfunc Other() int { return 1 }\n",
	"other/other_integration_test.go": "package other\n\nimport \"testing\"\n\nfunc TestOther(t *testing.T) { _ = Other() }\n",
}

// TestPipeline_GenThenTIA runs knit gen and knit tia end to end the way a
// user would: generate code, commit, change a dependency on a branch, then
// let tia select and execute the affected integration test.
func TestPipeline_GenThenTIA(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go and git")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	for rel, content := range pipelineModule {
		write(t, dir, rel, content)
	}
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOPROXY", "off")
	t.Chdir(dir)

	// gen -dry-run prints the code without writing it.
	code, out, errOut := runCLI(t, "gen", "-dry-run", "./...")
	if code != exitOK || !strings.Contains(out, "// ==> di/wire_gen.go <==\n// Code generated by knit. DO NOT EDIT.") {
		t.Fatalf("gen -dry-run = %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "di", "wire_gen.go")); !os.IsNotExist(err) {
		t.Fatalf("gen -dry-run wrote wire_gen.go (stat err = %v)", err)
	}

	code, _, errOut = runCLI(t, "gen")
	if code != exitOK || !strings.Contains(errOut, "wrote di/wire_gen.go") || !strings.Contains(errOut, "wrote .knit/di-graph.json") {
		t.Fatalf("gen = %d\nstderr: %s", code, errOut)
	}
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "base")
	git(t, dir, "checkout", "-q", "-b", "feature")

	// Change the store on the branch (uncommitted: -dirty defaults to true).
	write(t, dir, "store/store.go", strings.Replace(pipelineModule["store/store.go"], `return "world"`, `return "wor" + "ld"`, 1))

	code, out, errOut = runCLI(t, "tia", "-hub-exclude", "Initialize*", "-v")
	wantCmd := "go test -tags=integration -p 1 -count=1 -run '^(TestInitializeSvc)$' ./di\n"
	if code != exitOK || out != wantCmd {
		t.Fatalf("tia = %d\nstdout: %s\nstderr: %s\nwant stdout %q", code, out, errOut, wantCmd)
	}
	for _, want := range []string{"seed ./store.(*Store).Get", "test ./di.TestInitializeSvc"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("tia -v stderr lacks %q:\n%s", want, errOut)
		}
	}

	code, out, errOut = runCLI(t, "tia", "-exec")
	if code != exitOK || !strings.Contains(out, "ok  \texample.com/pipe/di") {
		t.Fatalf("tia -exec = %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}

	// A breaking change makes tia -exec propagate go test's exit status.
	write(t, dir, "store/store.go", strings.Replace(pipelineModule["store/store.go"], `return "world"`, `return "moon"`, 1))
	if code, out, errOut = runCLI(t, "tia", "-exec"); code != 1 {
		t.Fatalf("tia -exec (broken) = %d, want 1\nstdout: %s\nstderr: %s", code, out, errOut)
	}

	// Nothing changed relative to main: no tests.
	git(t, dir, "checkout", "-q", "--", ".")
	code, out, errOut = runCLI(t, "tia")
	if code != exitOK || out != "" || !strings.Contains(errOut, "no affected integration tests") {
		t.Fatalf("tia (clean) = %d\nstdout: %s\nstderr: %s", code, out, errOut)
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
	if code != exitError || !strings.Contains(errOut, "no injectors found") {
		t.Errorf("gen = %d, stderr: %s", code, errOut)
	}
}

func TestTIA_NoModule(t *testing.T) {
	t.Chdir(t.TempDir())
	code, _, errOut := runCLI(t, "tia")
	if code != exitError || !strings.Contains(errOut, "go.mod") {
		t.Errorf("tia = %d, stderr: %s", code, errOut)
	}
}
