package di

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"
)

const fixtureMod = "example.com/fixture"

// fixture loads testdata/fixture once per test binary; a single packages.Load
// keeps the whole suite well within the 1s budget for `go test -short`.
var fixture struct {
	once   sync.Once
	loader *Loader
	pkgs   map[string]*packages.Package
	err    error
}

func loadFixture(t *testing.T) (loader *Loader, pkgs map[string]*packages.Package) {
	t.Helper()
	fixture.once.Do(func() {
		dir, err := filepath.Abs(filepath.Join("testdata", "fixture"))
		if err != nil {
			fixture.err = err
			return
		}
		env := append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off")
		fixture.loader = NewLoader(WireMode, dir, env)
		pkgs, err := fixture.loader.Load(context.Background(), "./...")
		if err != nil {
			fixture.err = err
			return
		}
		fixture.pkgs = make(map[string]*packages.Package, len(pkgs))
		for _, p := range pkgs {
			fixture.pkgs[p.PkgPath] = p
		}
	})
	if fixture.err != nil {
		t.Fatalf("loading fixture: %v", fixture.err)
	}
	return fixture.loader, fixture.pkgs
}

// fixtureInjectors returns the injectors of the fixture package rel
// (relative to the fixture module root).
func fixtureInjectors(t *testing.T, rel string) ([]*InjectorSpec, error) {
	t.Helper()
	l, pkgs := loadFixture(t)
	pkg, ok := pkgs[fixtureMod+"/"+rel]
	if !ok {
		t.Fatalf("fixture package %q not loaded", rel)
	}
	return l.Injectors(context.Background(), pkg)
}

func mustOneInjector(t *testing.T, rel string) *InjectorSpec {
	t.Helper()
	injs, err := fixtureInjectors(t, rel)
	if err != nil {
		t.Fatalf("Injectors(%s) error = %v", rel, err)
	}
	if len(injs) != 1 {
		t.Fatalf("Injectors(%s) returned %d injectors, want 1", rel, len(injs))
	}
	return injs[0]
}

func providerIDs(ps []ProviderNode) []string {
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
	}
	return ids
}

// TestLoader_WireInjectTag verifies that wire.go guarded by
// //go:build wireinject is loaded while the stale wire_gen.go guarded by
// //go:build !wireinject is excluded.
func TestLoader_WireInjectTag(t *testing.T) {
	const p = fixtureMod + "/basic"
	inj := mustOneInjector(t, "basic")

	if inj.FuncName != "InitializeServer" || inj.PkgPath != p || inj.PkgName != "basic" {
		t.Errorf("injector = %s %s.%s, want basic %s.InitializeServer", inj.PkgName, inj.PkgPath, inj.FuncName, p)
	}
	if filepath.Base(inj.FilePath) != "wire.go" {
		t.Errorf("FilePath = %q, want .../wire.go (wire_gen.go must be excluded)", inj.FilePath)
	}
	wantParams := []ParamSpec{{Name: "cfg", Type: "*" + p + ".Config"}, {Name: "name", Type: "string"}}
	if !reflect.DeepEqual(inj.Params, wantParams) {
		t.Errorf("Params = %+v, want %+v", inj.Params, wantParams)
	}
	if inj.ReturnType != "*"+p+".Server" || !inj.HasCleanup || !inj.HasError {
		t.Errorf("result = (%s, cleanup=%v, error=%v), want (*%s.Server, true, true)", inj.ReturnType, inj.HasCleanup, inj.HasError, p)
	}

	// Nested wire.NewSet(StoreSet) is flattened; providers are sorted by ID.
	wantProviders := []ProviderNode{
		{
			ID: p + ".NewDB", PkgPath: p, PkgName: "basic", FuncName: "NewDB",
			Params:     []ParamSpec{{Name: "cfg", Type: "*" + p + ".Config"}},
			OutputType: "*" + p + ".DB", HasCleanup: true, HasError: true,
		},
		{
			ID: p + ".NewSQLStore", PkgPath: p, PkgName: "basic", FuncName: "NewSQLStore",
			Params:     []ParamSpec{{Name: "db", Type: "*" + p + ".DB"}},
			OutputType: "*" + p + ".sqlStore",
		},
		{
			ID: p + ".NewServer", PkgPath: p, PkgName: "basic", FuncName: "NewServer",
			Params:     []ParamSpec{{Name: "s", Type: p + ".Store"}, {Name: "name", Type: "string"}},
			OutputType: "*" + p + ".Server", HasError: true,
		},
	}
	if !reflect.DeepEqual(inj.Providers, wantProviders) {
		t.Errorf("Providers =\n%+v\nwant\n%+v", inj.Providers, wantProviders)
	}
	wantBindings := []BindingSpec{{InterfaceType: p + ".Store", ConcreteType: "*" + p + ".sqlStore"}}
	if !reflect.DeepEqual(inj.Bindings, wantBindings) {
		t.Errorf("Bindings = %+v, want %+v", inj.Bindings, wantBindings)
	}

	res, err := Resolve(inj)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := stepIDs(res), []string{p + ".NewDB", p + ".NewSQLStore", p + ".NewServer"}; !reflect.DeepEqual(got, want) {
		t.Errorf("steps = %v, want %v", got, want)
	}
	wantArgs := []ValueRef{
		{Type: p + ".Store", Kind: FromProvider, Index: 1},
		{Type: "string", Kind: FromParam, Index: 1},
	}
	if got := res.Steps[2].Args; !reflect.DeepEqual(got, wantArgs) {
		t.Errorf("NewServer args = %+v, want %+v", got, wantArgs)
	}
}

// TestLoader_SameNameTypes verifies that pkga.Config and pkgb.Config get
// distinct, fully qualified TypeKeys and resolve without collision, including
// a provider set declared in another package (pkga.Set).
func TestLoader_SameNameTypes(t *testing.T) {
	const (
		a   = fixtureMod + "/samename/pkga"
		b   = fixtureMod + "/samename/pkgb"
		app = fixtureMod + "/samename/app"
	)
	inj := mustOneInjector(t, "samename/di")

	want := map[string]TypeKey{
		a + ".NewConfig": "*" + a + ".Config",
		b + ".NewConfig": "*" + b + ".Config",
		app + ".NewApp":  "*" + app + ".App",
	}
	if len(inj.Providers) != len(want) {
		t.Fatalf("Providers = %v, want %d providers", providerIDs(inj.Providers), len(want))
	}
	for _, p := range inj.Providers {
		if p.OutputType != want[p.ID] {
			t.Errorf("%s OutputType = %q, want %q", p.ID, p.OutputType, want[p.ID])
		}
	}

	res, err := Resolve(inj)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, wantSteps := stepIDs(res), []string{a + ".NewConfig", b + ".NewConfig", app + ".NewApp"}; !reflect.DeepEqual(got, wantSteps) {
		t.Fatalf("steps = %v, want %v", got, wantSteps)
	}
	wantArgs := []ValueRef{
		{Type: "*" + a + ".Config", Kind: FromProvider, Index: 0},
		{Type: "*" + b + ".Config", Kind: FromProvider, Index: 1},
	}
	if got := res.Steps[2].Args; !reflect.DeepEqual(got, wantArgs) {
		t.Errorf("NewApp args = %+v, want %+v", got, wantArgs)
	}
}

// TestLoader_ResolveErrors verifies dependency-chain errors for injectors
// loaded from real source.
func TestLoader_ResolveErrors(t *testing.T) {
	const (
		cyc = fixtureMod + "/cycle"
		mis = fixtureMod + "/missing"
	)
	tests := []struct {
		name      string
		pkg       string
		wantKind  ResolveErrorKind
		wantChain []TypeKey
		wantMsg   string
	}{
		{
			name:      "circular dependency",
			pkg:       "cycle",
			wantKind:  ErrKindCircularDependency,
			wantChain: []TypeKey{"*" + cyc + ".A", "*" + cyc + ".B", "*" + cyc + ".C", "*" + cyc + ".A"},
			wantMsg: "failed to resolve provider for injector 'InitializeA':\n" +
				"  *cycle.A -> *cycle.B -> *cycle.C -> *cycle.A (circular dependency)",
		},
		{
			name:      "missing provider",
			pkg:       "missing/di",
			wantKind:  ErrKindMissingProvider,
			wantChain: []TypeKey{mis + "/service.UserService", mis + "/repo.UserRepository", "*" + mis + "/db.Pool"},
			wantMsg: "failed to resolve provider for injector 'InitializeUserService':\n" +
				"  service.UserService -> repo.UserRepository -> *db.Pool (provider not found)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(mustOneInjector(t, tt.pkg))
			var re *ResolveError
			if !errors.As(err, &re) {
				t.Fatalf("Resolve() error = %v, want *ResolveError", err)
			}
			if re.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", re.Kind, tt.wantKind)
			}
			if !reflect.DeepEqual(re.Chain, tt.wantChain) {
				t.Errorf("Chain = %v, want %v", re.Chain, tt.wantChain)
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("Error() =\n%s\nwant\n%s", err.Error(), tt.wantMsg)
			}
		})
	}
}

// TestLoader_BindAndDuplicateChains verifies the chains and explanations of
// conflicts found in real source.
func TestLoader_BindAndDuplicateChains(t *testing.T) {
	tests := []struct {
		pkg, injector string
		wantKind      ResolveErrorKind
		wantIn        []string
	}{
		{
			pkg: "errs/duplicate", injector: "InitializeService", wantKind: ErrKindDuplicateProvider,
			wantIn: []string{"*duplicate.Service -> *duplicate.Repo -> duplicate.Store (duplicate provider: Bind to "},
		},
		{
			pkg: "errs/badbind", injector: "InitializeService", wantKind: ErrKindInvalidBind,
			wantIn: []string{
				"*badbind.Service -> badbind.Store -> badbind.SQLStore (invalid Bind: ", "wire.go:",
				"badbind.SQLStore does not implement badbind.Store, but *badbind.SQLStore does; use new(*badbind.SQLStore)",
			},
		},
		{
			pkg: "errs/badbind", injector: "InitializeClosing", wantKind: ErrKindInvalidBind,
			wantIn: []string{"badbind.Other does not implement badbind.Closer: method Close has a different signature"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.pkg+"/"+tt.injector, func(t *testing.T) {
			injs, err := fixtureInjectors(t, tt.pkg)
			if err != nil {
				t.Fatal(err)
			}
			for _, inj := range injs {
				if inj.FuncName != tt.injector {
					continue
				}
				_, err := Resolve(inj)
				var re *ResolveError
				if !errors.As(err, &re) || re.Kind != tt.wantKind {
					t.Fatalf("Resolve() error = %v, want kind %v", err, tt.wantKind)
				}
				if tt.wantKind == ErrKindInvalidBind && !errors.Is(err, ErrInvalidBind) {
					t.Errorf("errors.Is(%v, ErrInvalidBind) = false", err)
				}
				for _, s := range tt.wantIn {
					if !strings.Contains(err.Error(), s) {
						t.Errorf("error lacks %q:\n%v", s, err)
					}
				}
				return
			}
			t.Fatalf("injector %s not found", tt.injector)
		})
	}
}

// TestLoader_Rejects verifies that variadic parameters and out-of-scope wire
// features produce clear errors instead of panics.
func TestLoader_Rejects(t *testing.T) {
	tests := []struct {
		name      string
		pkg       string
		wantIs    error
		wantInMsg []string
	}{
		{
			name:   "variadic provider and injector",
			pkg:    "variadic",
			wantIs: ErrVariadic,
			wantInMsg: []string{
				"injector InitializeClient:",
				"provider " + fixtureMod + "/variadic.NewClient: variadic parameters are not supported: parameter opts ...variadic.Option",
				"injector InitializeClientWithOptions: variadic parameters are not supported: parameter opts ...variadic.Option",
				"[]variadic.Option",
				"wire.go:",
			},
		},
		{
			name:      "wire.Value is out of scope",
			pkg:       "unsupported",
			wantIs:    ErrUnsupported,
			wantInMsg: []string{"injector InitializeService:", "wire.Value is not supported by knit"},
		},
		{
			name:      "injector file without the inject tag",
			pkg:       "invalid/notag",
			wantIs:    ErrMissingBuildTag,
			wantInMsg: []string{"injector InitializeT:", "wire.go:", "//go:build wireinject"},
		},
		{
			name:      "provider declared under the inject tag",
			pkg:       "invalid/tagged",
			wantIs:    ErrInaccessibleProvider,
			wantInMsg: []string{"injector InitializeT:", "tagged.NewT", "wire.go is only compiled with the wireinject tag"},
		},
		{
			name:      "unexported provider of another package",
			pkg:       "invalid/unexported/di",
			wantIs:    ErrInaccessibleProvider,
			wantInMsg: []string{"injector InitializeT:", "lib.newT", "it is unexported"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				injs []*InjectorSpec
				err  error
			)
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("Injectors() panicked: %v", r)
					}
				}()
				injs, err = fixtureInjectors(t, tt.pkg)
			}()
			if err == nil {
				t.Fatalf("Injectors() = %d injectors, want error", len(injs))
			}
			if !errors.Is(err, tt.wantIs) {
				t.Errorf("Injectors() error = %v, want errors.Is %v", err, tt.wantIs)
			}
			for _, s := range tt.wantInMsg {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("error message does not contain %q:\n%s", s, err.Error())
				}
			}
		})
	}
}

// TestLoadInjectors_LazyCrossPackageSet verifies the convenience entry point
// and lazy loading of a provider set declared in a package outside the
// requested patterns (samename/pkga.Set).
func TestLoadInjectors_LazyCrossPackageSet(t *testing.T) {
	if testing.Short() {
		// Each standalone load spawns `go list`; the shared-load tests above
		// already cover the analysis in -short mode.
		t.Skip("spawns extra go list processes")
	}
	dir, _ := filepath.Abs(filepath.Join("testdata", "fixture"))
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOPROXY", "off")
	injs, err := LoadInjectors(context.Background(), WireMode, dir, "./samename/di")
	if err != nil {
		t.Fatalf("LoadInjectors() error = %v", err)
	}
	if len(injs) != 1 || len(injs[0].Providers) != 3 {
		t.Fatalf("LoadInjectors() = %+v, want 1 injector with 3 providers", injs)
	}
}
