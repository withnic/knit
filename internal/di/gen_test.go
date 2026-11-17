package di

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/withnic/knit/internal/graphschema"
)

// copyDir copies the directory tree src into dst.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatalf("copying %s: %v", src, err)
	}
}

func goCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// TestGen_EndToEnd generates code into a temporary copy of the fixture module,
// then builds, vets and runs it: the LIFO cleanup behavior is verified by
// executing testdata/fixture/gen/lifo/lifo_test.go against the generated code.
func TestGen_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns go build / go test")
	}
	root := t.TempDir()
	copyDir(t, filepath.Join("testdata", "fixture"), filepath.Join(root, "fixture"))
	copyDir(t, filepath.Join("testdata", "wire"), filepath.Join(root, "wire"))
	dir := filepath.Join(root, "fixture")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOPROXY", "off")

	opts := GenOptions{
		Dir:      dir,
		Patterns: []string{"./basic", "./samename/...", "./gen/..."},
		Now:      func() time.Time { return time.Unix(0, 0).UTC() },
	}
	res, err := Gen(context.Background(), opts)
	if err != nil {
		t.Fatalf("Gen() error = %v", err)
	}
	var rels []string
	for _, f := range res.Files {
		rel, _ := filepath.Rel(dir, f.Path)
		rels = append(rels, filepath.ToSlash(rel))
	}
	wantFiles := []string{"basic/wire_gen.go", "gen/collide/app/wire_gen.go", "gen/lifo/wire_gen.go", "samename/di/wire_gen.go"}
	if strings.Join(rels, ",") != strings.Join(wantFiles, ",") {
		t.Fatalf("generated files = %v, want %v", rels, wantFiles)
	}

	graphPath := filepath.Join(dir, graphschema.DefaultPath)
	if err := res.Write(graphPath); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	g, err := graphschema.ReadFile(graphPath)
	if err != nil {
		t.Fatalf("reading written graph: %v", err)
	}
	if len(g.Injectors) != 5 {
		t.Errorf("graph has %d injectors, want 5", len(g.Injectors))
	}

	// Regenerating with the generated files in place is byte-identical.
	again, err := Gen(context.Background(), opts)
	if err != nil {
		t.Fatalf("second Gen() error = %v", err)
	}
	for i := range res.Files {
		if !bytes.Equal(res.Files[i].Content, again.Files[i].Content) {
			t.Errorf("%s differs between runs", rels[i])
		}
	}

	goCmd(t, dir, "build", "./...")
	goCmd(t, dir, "vet", "./basic", "./samename/...", "./gen/...")
	goCmd(t, dir, "test", "-count=1", "./gen/lifo")
}

func TestGen_Errors(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns go list")
	}
	dir, _ := filepath.Abs(filepath.Join("testdata", "fixture"))
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOPROXY", "off")
	tests := []struct {
		name string
		opts GenOptions
		want []string
	}{
		{
			name: "all resolve errors are reported",
			opts: GenOptions{Dir: dir, Patterns: []string{"./cycle", "./missing/..."}},
			want: []string{"injector 'InitializeA'", "circular dependency", "injector 'InitializeUserService'", "provider not found"},
		},
		{
			name: "no injectors",
			opts: GenOptions{Dir: dir, Patterns: []string{"./missing/db"}},
			want: []string{"no injectors found"},
		},
		{
			name: "invalid output file",
			opts: GenOptions{Dir: dir, OutFile: "sub/wire_gen.go"},
			want: []string{"must be a plain .go file name"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Gen(context.Background(), tt.opts)
			if err == nil {
				t.Fatal("Gen() error = nil")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("Gen() error lacks %q:\n%v", w, err)
				}
			}
		})
	}
}
