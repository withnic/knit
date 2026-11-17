package di

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// prov builds an in-memory provider "<pkg>.<fn>" producing out from params.
func prov(id string, out TypeKey, params ...TypeKey) ProviderNode {
	i := strings.LastIndex(id, ".")
	p := ProviderNode{ID: id, PkgPath: id[:i], PkgName: id[strings.LastIndex(id[:i], "/")+1 : i], FuncName: id[i+1:], OutputType: out}
	for _, t := range params {
		p.Params = append(p.Params, ParamSpec{Type: t})
	}
	return p
}

func stepIDs(r *ResolvedInjector) []string {
	ids := make([]string, len(r.Steps))
	for i, s := range r.Steps {
		ids[i] = s.Provider.ID
	}
	return ids
}

func TestResolve_TopologicalOrder(t *testing.T) {
	const (
		cfg    TypeKey = "*ex.com/cfg.Config"
		db     TypeKey = "*ex.com/db.Pool"
		cache  TypeKey = "*ex.com/cache.Cache"
		repo   TypeKey = "*ex.com/repo.Repo"
		svc    TypeKey = "ex.com/svc.Service"
		svcImp TypeKey = "*ex.com/svc.impl"
		srv    TypeKey = "*ex.com/srv.Server"
		ctx    TypeKey = "context.Context"
	)
	tests := []struct {
		name      string
		inj       InjectorSpec
		wantSteps []string
		wantArgs  map[string][]ValueRef // provider ID -> args
		wantRoot  ValueRef
	}{
		{
			name: "linear chain",
			inj: InjectorSpec{FuncName: "Init", ReturnType: repo, Providers: []ProviderNode{
				prov("ex.com/repo.NewRepo", repo, db),
				prov("ex.com/db.NewPool", db, cfg),
				prov("ex.com/cfg.NewConfig", cfg),
			}},
			wantSteps: []string{"ex.com/cfg.NewConfig", "ex.com/db.NewPool", "ex.com/repo.NewRepo"},
			wantArgs: map[string][]ValueRef{
				"ex.com/db.NewPool":   {{Type: cfg, Kind: FromProvider, Index: 0}},
				"ex.com/repo.NewRepo": {{Type: db, Kind: FromProvider, Index: 1}},
			},
			wantRoot: ValueRef{Type: repo, Kind: FromProvider, Index: 2},
		},
		{
			name: "diamond with ID tie-break and shared dependency built once",
			inj: InjectorSpec{FuncName: "Init", ReturnType: srv, Providers: []ProviderNode{
				prov("ex.com/srv.NewServer", srv, repo, cache),
				prov("ex.com/repo.NewRepo", repo, cfg),
				prov("ex.com/cache.NewCache", cache, cfg),
				prov("ex.com/cfg.NewConfig", cfg),
			}},
			wantSteps: []string{"ex.com/cfg.NewConfig", "ex.com/cache.NewCache", "ex.com/repo.NewRepo", "ex.com/srv.NewServer"},
			wantArgs: map[string][]ValueRef{
				"ex.com/srv.NewServer": {
					{Type: repo, Kind: FromProvider, Index: 2},
					{Type: cache, Kind: FromProvider, Index: 1},
				},
			},
			wantRoot: ValueRef{Type: srv, Kind: FromProvider, Index: 3},
		},
		{
			name: "injector params and binding; unused providers are skipped",
			inj: InjectorSpec{
				FuncName:   "Init",
				Params:     []ParamSpec{{Name: "ctx", Type: ctx}, {Name: "pool", Type: db}},
				ReturnType: srv,
				Providers: []ProviderNode{
					prov("ex.com/srv.NewServer", srv, ctx, svc),
					prov("ex.com/svc.NewImpl", svcImp, db),
					prov("ex.com/cache.NewCache", cache), // unused
				},
				Bindings: []BindingSpec{{InterfaceType: svc, ConcreteType: svcImp}},
			},
			wantSteps: []string{"ex.com/svc.NewImpl", "ex.com/srv.NewServer"},
			wantArgs: map[string][]ValueRef{
				"ex.com/svc.NewImpl": {{Type: db, Kind: FromParam, Index: 1}},
				"ex.com/srv.NewServer": {
					{Type: ctx, Kind: FromParam, Index: 0},
					{Type: svc, Kind: FromProvider, Index: 0, BoundFrom: svcImp},
				},
			},
			wantRoot: ValueRef{Type: srv, Kind: FromProvider, Index: 1},
		},
		{
			name: "root bound to injector parameter",
			inj: InjectorSpec{
				FuncName:   "Init",
				Params:     []ParamSpec{{Name: "impl", Type: svcImp}},
				ReturnType: svc,
				Bindings:   []BindingSpec{{InterfaceType: svc, ConcreteType: svcImp}},
			},
			wantSteps: []string{},
			wantRoot:  ValueRef{Type: svc, Kind: FromParam, Index: 0, BoundFrom: svcImp},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(&tt.inj)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if ids := stepIDs(got); !reflect.DeepEqual(ids, tt.wantSteps) {
				t.Errorf("steps = %v, want %v", ids, tt.wantSteps)
			}
			for _, s := range got.Steps {
				if want, ok := tt.wantArgs[s.Provider.ID]; ok && !reflect.DeepEqual(s.Args, want) {
					t.Errorf("args of %s = %+v, want %+v", s.Provider.ID, s.Args, want)
				}
			}
			if got.Root != tt.wantRoot {
				t.Errorf("root = %+v, want %+v", got.Root, tt.wantRoot)
			}
			assertTopological(t, got)
		})
	}
}

// assertTopological checks that every provider argument refers to an earlier step.
func assertTopological(t *testing.T, r *ResolvedInjector) {
	t.Helper()
	for i, s := range r.Steps {
		for _, a := range s.Args {
			if a.Kind == FromProvider && a.Index >= i {
				t.Errorf("step %d (%s) depends on later step %d", i, s.Provider.ID, a.Index)
			}
		}
	}
}

func TestResolve_Deterministic(t *testing.T) {
	providers := []ProviderNode{
		prov("ex.com/z.NewZ", "ex.com/z.Z", "ex.com/a.A", "ex.com/b.B", "ex.com/c.C"),
		prov("ex.com/c.NewC", "ex.com/c.C"),
		prov("ex.com/a.NewA", "ex.com/a.A"),
		prov("ex.com/b.NewB", "ex.com/b.B", "ex.com/a.A"),
	}
	var first []string
	for i := 0; i < 20; i++ {
		// Rotate the input order; the output must not change.
		ps := append(append([]ProviderNode(nil), providers[i%len(providers):]...), providers[:i%len(providers)]...)
		got, err := Resolve(&InjectorSpec{FuncName: "Init", ReturnType: "ex.com/z.Z", Providers: ps})
		if err != nil {
			t.Fatal(err)
		}
		ids := stepIDs(got)
		if first == nil {
			first = ids
			continue
		}
		if !reflect.DeepEqual(ids, first) {
			t.Fatalf("run %d: steps = %v, want %v", i, ids, first)
		}
	}
	want := []string{"ex.com/a.NewA", "ex.com/b.NewB", "ex.com/c.NewC", "ex.com/z.NewZ"}
	if !reflect.DeepEqual(first, want) {
		t.Errorf("steps = %v, want %v", first, want)
	}
}

func TestResolve_Errors(t *testing.T) {
	tests := []struct {
		name      string
		inj       InjectorSpec
		wantKind  ResolveErrorKind
		wantChain []TypeKey
		wantMsg   string // exact Error() output when non-empty
		wantInMsg []string
	}{
		{
			name: "missing provider reports full chain",
			inj: InjectorSpec{FuncName: "InitializeUserService", ReturnType: "ex.com/service.UserService", Providers: []ProviderNode{
				prov("ex.com/service.NewUserService", "ex.com/service.UserService", "ex.com/repo.UserRepository"),
				prov("ex.com/repo.NewUserRepository", "ex.com/repo.UserRepository", "*ex.com/db.Pool"),
			}},
			wantKind:  ErrKindMissingProvider,
			wantChain: []TypeKey{"ex.com/service.UserService", "ex.com/repo.UserRepository", "*ex.com/db.Pool"},
			wantMsg: "failed to resolve provider for injector 'InitializeUserService':\n" +
				"  service.UserService -> repo.UserRepository -> *db.Pool (provider not found)",
		},
		{
			name:      "missing root provider",
			inj:       InjectorSpec{FuncName: "Init", ReturnType: "*ex.com/a.A"},
			wantKind:  ErrKindMissingProvider,
			wantChain: []TypeKey{"*ex.com/a.A"},
		},
		{
			name: "circular dependency reports cycle path",
			inj: InjectorSpec{FuncName: "InitA", ReturnType: "*ex.com/x.Root", Providers: []ProviderNode{
				prov("ex.com/x.NewRoot", "*ex.com/x.Root", "*ex.com/x.A"),
				prov("ex.com/x.NewA", "*ex.com/x.A", "*ex.com/x.B"),
				prov("ex.com/x.NewB", "*ex.com/x.B", "*ex.com/x.C"),
				prov("ex.com/x.NewC", "*ex.com/x.C", "*ex.com/x.A"),
			}},
			wantKind:  ErrKindCircularDependency,
			wantChain: []TypeKey{"*ex.com/x.Root", "*ex.com/x.A", "*ex.com/x.B", "*ex.com/x.C", "*ex.com/x.A"},
			wantMsg: "failed to resolve provider for injector 'InitA':\n" +
				"  *x.Root -> *x.A -> *x.B -> *x.C -> *x.A (circular dependency)",
		},
		{
			name: "self dependency",
			inj: InjectorSpec{FuncName: "Init", ReturnType: "*ex.com/x.A", Providers: []ProviderNode{
				prov("ex.com/x.NewA", "*ex.com/x.A", "*ex.com/x.A"),
			}},
			wantKind:  ErrKindCircularDependency,
			wantChain: []TypeKey{"*ex.com/x.A", "*ex.com/x.A"},
		},
		{
			name: "cycle through binding",
			inj: InjectorSpec{
				FuncName:   "Init",
				ReturnType: "ex.com/x.I",
				Providers:  []ProviderNode{prov("ex.com/x.NewImpl", "*ex.com/x.impl", "ex.com/x.I")},
				Bindings:   []BindingSpec{{InterfaceType: "ex.com/x.I", ConcreteType: "*ex.com/x.impl"}},
			},
			wantKind:  ErrKindCircularDependency,
			wantChain: []TypeKey{"ex.com/x.I", "*ex.com/x.impl", "ex.com/x.I"},
		},
		{
			name: "duplicate providers for the same type",
			inj: InjectorSpec{FuncName: "Init", ReturnType: "*ex.com/x.A", Providers: []ProviderNode{
				prov("ex.com/x.NewA", "*ex.com/x.A"),
				prov("ex.com/x.NewA2", "*ex.com/x.A"),
			}},
			wantKind:  ErrKindDuplicateProvider,
			wantChain: []TypeKey{"*ex.com/x.A"},
			wantInMsg: []string{"ex.com/x.NewA, ex.com/x.NewA2"},
		},
		{
			name: "provider conflicts with injector parameter",
			inj: InjectorSpec{
				FuncName:   "Init",
				Params:     []ParamSpec{{Name: "a", Type: "*ex.com/x.A"}},
				ReturnType: "*ex.com/x.A",
				Providers:  []ProviderNode{prov("ex.com/x.NewA", "*ex.com/x.A")},
			},
			wantKind:  ErrKindDuplicateProvider,
			wantChain: []TypeKey{"*ex.com/x.A"},
			wantInMsg: []string{"injector parameter a"},
		},
		{
			name: "provider error requires injector error",
			inj: InjectorSpec{FuncName: "Init", ReturnType: "*ex.com/x.A", Providers: []ProviderNode{
				func() ProviderNode { p := prov("ex.com/x.NewA", "*ex.com/x.A"); p.HasError = true; return p }(),
			}},
			wantKind:  ErrKindMissingError,
			wantChain: []TypeKey{"*ex.com/x.A"},
		},
		{
			name: "provider cleanup requires injector cleanup",
			inj: InjectorSpec{FuncName: "Init", HasError: true, ReturnType: "*ex.com/x.B", Providers: []ProviderNode{
				prov("ex.com/x.NewB", "*ex.com/x.B", "*ex.com/x.A"),
				func() ProviderNode { p := prov("ex.com/x.NewA", "*ex.com/x.A"); p.HasCleanup = true; return p }(),
			}},
			wantKind:  ErrKindMissingCleanup,
			wantChain: []TypeKey{"*ex.com/x.B", "*ex.com/x.A"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(&tt.inj)
			var re *ResolveError
			if !errors.As(err, &re) {
				t.Fatalf("Resolve() error = %v, want *ResolveError", err)
			}
			if re.Injector != tt.inj.FuncName {
				t.Errorf("Injector = %q, want %q", re.Injector, tt.inj.FuncName)
			}
			if re.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", re.Kind, tt.wantKind)
			}
			if !reflect.DeepEqual(re.Chain, tt.wantChain) {
				t.Errorf("Chain = %v, want %v", re.Chain, tt.wantChain)
			}
			msg := err.Error()
			if tt.wantMsg != "" && msg != tt.wantMsg {
				t.Errorf("Error() =\n%s\nwant\n%s", msg, tt.wantMsg)
			}
			if !strings.HasPrefix(msg, "failed to resolve provider for injector '"+tt.inj.FuncName+"':\n  ") {
				t.Errorf("Error() lacks injector context: %q", msg)
			}
			for _, s := range tt.wantInMsg {
				if !strings.Contains(msg, s) {
					t.Errorf("Error() = %q, want it to contain %q", msg, s)
				}
			}
		})
	}
}
