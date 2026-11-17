package di

import (
	"bytes"
	"errors"
	"flag"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata/golden")

// resolveFixture loads and resolves every injector of the fixture package rel.
func resolveFixture(t *testing.T, rel string) ([]*ResolvedInjector, *packages.Package) {
	t.Helper()
	_, pkgs := loadFixture(t)
	pkg := pkgs[fixtureMod+"/"+rel]
	injs, err := fixtureInjectors(t, rel)
	if err != nil {
		t.Fatalf("Injectors(%s) error = %v", rel, err)
	}
	out := make([]*ResolvedInjector, len(injs))
	for i, inj := range injs {
		r, err := Resolve(inj)
		if err != nil {
			t.Fatalf("Resolve(%s) error = %v", inj.FuncName, err)
		}
		out[i] = r
	}
	return out, pkg
}

// TestGenerateFile_Golden checks the generated code against golden files,
// that it is deterministic, carries the required header, and type-checks
// together with the rest of its package.
func TestGenerateFile_Golden(t *testing.T) {
	tests := []struct {
		pkg    string
		golden string
	}{
		// Bind, injector parameters, (T, func(), error) provider.
		{pkg: "basic", golden: "basic.golden"},
		// Same-named types from different packages, no error/cleanup.
		{pkg: "samename/di", golden: "samename.golden"},
		// Three cleanup+error providers: LIFO cleanup on failure.
		{pkg: "gen/lifo", golden: "lifo.golden"},
		// Two packages named "config", a package named "v1", a parameter
		// named "config", a package-level "cleanup", value-type zero value,
		// stdlib import grouping and two source files.
		{pkg: "gen/collide/app", golden: "collide.golden"},
	}
	for _, tt := range tests {
		t.Run(tt.pkg, func(t *testing.T) {
			injs, pkg := resolveFixture(t, tt.pkg)
			got, err := GenerateFile(injs)
			if err != nil {
				t.Fatalf("GenerateFile() error = %v", err)
			}

			// Deterministic: a second run with reversed input is identical.
			rev := make([]*ResolvedInjector, len(injs))
			for i := range injs {
				rev[len(injs)-1-i] = injs[i]
			}
			again, err := GenerateFile(rev)
			if err != nil {
				t.Fatalf("GenerateFile() second run error = %v", err)
			}
			if !bytes.Equal(got, again) {
				t.Errorf("GenerateFile() is not deterministic:\n%s\n---\n%s", got, again)
			}

			if !bytes.HasPrefix(got, []byte(GeneratedHeader+"\n")) {
				t.Errorf("generated code does not start with %q", GeneratedHeader)
			}
			if !bytes.Contains(got, []byte("\n//go:build !wireinject\n")) {
				t.Error("generated code lacks //go:build !wireinject")
			}

			typeCheckGenerated(t, pkg, got)

			path := filepath.Join("testdata", "golden", tt.golden)
			if *update {
				if err := os.WriteFile(path, got, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading golden file (run with -update to create it): %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("generated code differs from %s (run with -update to accept):\n%s", path, got)
			}
		})
	}
}

// typeCheckGenerated type-checks src together with the files of pkg that are
// compiled without the wireinject tag, i.e. the package as it is built in
// production.
func typeCheckGenerated(t *testing.T, pkg *packages.Package, src []byte) {
	t.Helper()
	files := make([]*ast.File, 0, len(pkg.Syntax)+1)
	for _, f := range pkg.Syntax {
		if !requiresWireInject(f) {
			files = append(files, f)
		}
	}
	gen, err := parser.ParseFile(pkg.Fset, DefaultOutFile, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing generated code: %v\n%s", err, src)
	}
	files = append(files, gen)

	deps := make(map[string]*types.Package)
	var collect func(p *types.Package)
	collect = func(p *types.Package) {
		for _, imp := range p.Imports() {
			if _, ok := deps[imp.Path()]; !ok {
				deps[imp.Path()] = imp
				collect(imp)
			}
		}
	}
	collect(pkg.Types)

	conf := types.Config{Importer: importerFunc(func(path string) (*types.Package, error) {
		if p, ok := deps[path]; ok {
			return p, nil
		}
		return nil, errors.New("package not loaded: " + path)
	})}
	if _, err := conf.Check(pkg.PkgPath, pkg.Fset, files, nil); err != nil {
		t.Fatalf("generated code does not type-check: %v\n%s", err, src)
	}
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }

// requiresWireInject reports whether f has a //go:build constraint that is
// only satisfied with the wireinject tag.
func requiresWireInject(f *ast.File) bool {
	for _, cg := range f.Comments {
		if cg.Pos() > f.Package {
			break
		}
		for _, c := range cg.List {
			if !constraint.IsGoBuild(c.Text) {
				continue
			}
			expr, err := constraint.Parse(c.Text)
			if err != nil {
				continue
			}
			without := expr.Eval(func(string) bool { return false })
			with := expr.Eval(func(tag string) bool { return tag == wireInjectTag })
			return with && !without
		}
	}
	return false
}

func TestGenerateFile_Errors(t *testing.T) {
	injs, _ := resolveFixture(t, "basic")
	other, _ := resolveFixture(t, "gen/lifo")
	tests := []struct {
		name string
		injs []*ResolvedInjector
		want string
	}{
		{name: "no injectors", injs: nil, want: "no injectors"},
		{
			name: "missing type information",
			injs: []*ResolvedInjector{{Injector: &InjectorSpec{FuncName: "Init"}}},
			want: "missing type information",
		},
		{name: "mixed packages", injs: append(injs[:1:1], other...), want: "different packages"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GenerateFile(tt.injs)
			if !errors.Is(err, ErrGenerate) || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("GenerateFile() error = %v, want ErrGenerate containing %q", err, tt.want)
			}
		})
	}
}

func TestZeroValue(t *testing.T) {
	pkg := types.NewPackage("ex.com/p", "p")
	named := func(name string, u types.Type) types.Type {
		return types.NewNamed(types.NewTypeName(0, pkg, name, nil), u, nil)
	}
	q := func(p *types.Package) string { return p.Name() }
	tests := []struct {
		typ  types.Type
		want string
	}{
		{typ: types.Typ[types.Bool], want: "false"},
		{typ: types.Typ[types.String], want: `""`},
		{typ: types.Typ[types.Int64], want: "0"},
		{typ: named("ID", types.Typ[types.Int]), want: "0"},
		{typ: types.NewPointer(types.Typ[types.Int]), want: "nil"},
		{typ: types.NewSlice(types.Typ[types.Int]), want: "nil"},
		{typ: types.NewMap(types.Typ[types.String], types.Typ[types.Int]), want: "nil"},
		{typ: named("Svc", types.NewInterfaceType(nil, nil)), want: "nil"},
		{typ: named("App", types.NewStruct(nil, nil)), want: "p.App{}"},
		{typ: types.NewArray(types.Typ[types.Int], 2), want: "[2]int{}"},
	}
	for _, tt := range tests {
		t.Run(tt.typ.String(), func(t *testing.T) {
			if got := zeroValue(tt.typ, q); got != tt.want {
				t.Errorf("zeroValue(%s) = %s, want %s", tt.typ, got, tt.want)
			}
		})
	}
}

func TestNamer(t *testing.T) {
	n := newNamer(map[string]bool{"cleanup": true, "v2": true})
	got := []string{n.take("cleanup"), n.take("cleanup"), n.seq("v"), n.seq("v"), n.take("type"), n.take("arg")}
	want := []string{"cleanup1", "cleanup2", "v1", "v3", "type1", "arg"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("names = %v, want %v", got, want)
	}
}
