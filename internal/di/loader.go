package di

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// WirePkgPath is the import path of the wire marker package whose
// Build / NewSet / Bind calls knit understands.
const WirePkgPath = "github.com/google/wire"

// wireInjectTag is the build tag that enables injector declaration files.
const wireInjectTag = "wireinject"

// loadMode is the minimal packages.Load mode needed to analyze injectors.
const loadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedSyntax |
	packages.NeedTypes |
	packages.NeedTypesInfo |
	packages.NeedImports

// Sentinel errors that callers (and tests) can match with errors.Is.
var (
	// ErrVariadic is returned when a provider or injector has a variadic (...T)
	// parameter. Variadic parameters cannot be expressed as a TypeKey.
	ErrVariadic = errors.New("variadic parameters are not supported")
	// ErrUnsupported is returned for wire features knit deliberately does not
	// implement (wire.Struct, wire.Value, wire.InterfaceValue, wire.FieldsOf)
	// and other unsupported constructs.
	ErrUnsupported = errors.New("unsupported construct")
	// ErrInvalidSignature is returned when a provider or injector has a result
	// signature other than (T), (T, error), (T, func()) or (T, func(), error).
	ErrInvalidSignature = errors.New("invalid signature")
	// ErrInvalidBind is returned for malformed wire.Bind calls.
	ErrInvalidBind = errors.New("invalid wire.Bind")
)

// unsupportedWireFuncs lists wire functions that are intentionally out of scope
// (docs/01_system_overview.md §2.1). They must be rejected, not implemented.
var unsupportedWireFuncs = map[string]string{
	"Struct":         "write an explicit NewXxx constructor instead",
	"Value":          "write a one-line ProvideXxx function instead",
	"InterfaceValue": "write a one-line ProvideXxx function instead",
	"FieldsOf":       "write a one-line getter function instead",
}

// Loader loads Go packages with the wireinject build tag and extracts injector
// specifications from them.
type Loader struct {
	cfg *packages.Config
	// pkgs caches packages loaded with full syntax, keyed by import path.
	pkgs map[string]*packages.Package
}

// NewLoader returns a Loader that resolves package patterns relative to dir.
// env, when non-nil, replaces the environment of the underlying go command.
func NewLoader(dir string, env []string) *Loader {
	if env == nil {
		env = os.Environ()
	}
	return &Loader{
		cfg: &packages.Config{
			Mode:       loadMode,
			Dir:        dir,
			Env:        env,
			Fset:       token.NewFileSet(),
			BuildFlags: []string{"-tags=" + wireInjectTag},
			Tests:      false,
		},
		pkgs: make(map[string]*packages.Package),
	}
}

// Load loads the packages matching patterns and returns them sorted by import
// path. Any package or type-checking error aborts the load.
func (l *Loader) Load(ctx context.Context, patterns ...string) ([]*packages.Package, error) {
	cfg := *l.cfg
	cfg.Context = ctx
	pkgs, err := packages.Load(&cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages %v: %w", patterns, err)
	}
	var msgs []string
	for _, p := range pkgs {
		for _, e := range p.Errors {
			msgs = append(msgs, e.Error())
		}
	}
	if len(msgs) > 0 {
		sort.Strings(msgs)
		return nil, fmt.Errorf("loading packages %v:\n  %s", patterns, strings.Join(msgs, "\n  "))
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("loading packages %v: no packages found", patterns)
	}
	for _, p := range pkgs {
		l.pkgs[p.PkgPath] = p
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].PkgPath < pkgs[j].PkgPath })
	return pkgs, nil
}

// pkgWithSyntax returns the package for path with syntax and type info,
// loading it lazily when it was not part of a previous Load call.
func (l *Loader) pkgWithSyntax(ctx context.Context, path string) (*packages.Package, error) {
	if p, ok := l.pkgs[path]; ok {
		return p, nil
	}
	pkgs, err := l.Load(ctx, path)
	if err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		if p.PkgPath == path {
			return p, nil
		}
	}
	return nil, fmt.Errorf("package %q not found", path)
}

// LoadInjectors is a convenience wrapper that loads patterns from dir and
// returns every injector found, sorted by package path and function name.
func LoadInjectors(ctx context.Context, dir string, patterns ...string) ([]*InjectorSpec, error) {
	l := NewLoader(dir, nil)
	pkgs, err := l.Load(ctx, patterns...)
	if err != nil {
		return nil, err
	}
	var all []*InjectorSpec
	for _, p := range pkgs {
		injs, err := l.Injectors(ctx, p)
		if err != nil {
			return nil, err
		}
		all = append(all, injs...)
	}
	return all, nil
}

// Injectors extracts every injector function declared in pkg, i.e. every
// top-level function whose body calls wire.Build. The result is sorted by
// function name. All errors found in the package are reported together.
func (l *Loader) Injectors(ctx context.Context, pkg *packages.Package) ([]*InjectorSpec, error) {
	var (
		injs []*InjectorSpec
		errs []error
	)
	for _, file := range pkg.Syntax {
		filePath := pkg.Fset.Position(file.Package).Filename
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			builds := findWireCalls(pkg.TypesInfo, fn.Body, "Build")
			if len(builds) == 0 {
				continue
			}
			inj, err := l.injector(ctx, pkg, filePath, fn, builds)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			injs = append(injs, inj)
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	sort.Slice(injs, func(i, j int) bool { return injs[i].FuncName < injs[j].FuncName })
	return injs, nil
}

func (l *Loader) injector(ctx context.Context, pkg *packages.Package, filePath string, fn *ast.FuncDecl, builds []*ast.CallExpr) (*InjectorSpec, error) {
	name := fn.Name.Name
	pos := func(p token.Pos) string { return pkg.Fset.Position(p).String() }

	if fn.Recv != nil {
		return nil, fmt.Errorf("%s: injector %s: %w: injectors must not be methods", pos(fn.Pos()), name, ErrUnsupported)
	}
	if len(builds) > 1 {
		return nil, fmt.Errorf("%s: injector %s: %w: multiple wire.Build calls", pos(builds[1].Pos()), name, ErrUnsupported)
	}
	obj, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func)
	if !ok {
		return nil, fmt.Errorf("%s: injector %s: missing type information", pos(fn.Pos()), name)
	}
	sig := obj.Signature()
	if sig.TypeParams().Len() > 0 {
		return nil, fmt.Errorf("%s: injector %s: %w: generic injectors", pos(fn.Pos()), name, ErrUnsupported)
	}
	params, err := paramSpecs(sig)
	if err != nil {
		return nil, fmt.Errorf("%s: injector %s: %w", pos(fn.Pos()), name, err)
	}
	seen := make(map[TypeKey]string, len(params))
	for _, p := range params {
		if prev, dup := seen[p.Type]; dup {
			return nil, fmt.Errorf("%s: injector %s: parameters %q and %q have the same type %s",
				pos(fn.Pos()), name, prev, p.Name, p.Type)
		}
		seen[p.Type] = p.Name
	}
	out, hasCleanup, hasErr, err := classifyResults(sig.Results())
	if err != nil {
		return nil, fmt.Errorf("%s: injector %s: %w", pos(fn.Pos()), name, err)
	}

	sb := newSetBuilder(ctx, l)
	for _, arg := range builds[0].Args {
		sb.addExpr(pkg, arg)
	}
	if len(sb.errs) > 0 {
		return nil, fmt.Errorf("injector %s: %w", name, errors.Join(sb.errs...))
	}

	return &InjectorSpec{
		FuncName:   name,
		PkgPath:    pkg.PkgPath,
		PkgName:    pkg.Name,
		FilePath:   filePath,
		Params:     params,
		ReturnType: NewTypeKey(out),
		HasCleanup: hasCleanup,
		HasError:   hasErr,
		Providers:  sb.sortedProviders(),
		Bindings:   sb.sortedBindings(),
	}, nil
}

// findWireCalls returns every call to wire.<name> inside root, in source order.
func findWireCalls(info *types.Info, root ast.Node, name string) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(root, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if fn, ok := wireFuncName(info, call.Fun); ok && fn == name {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

// wireFuncName reports the wire function referenced by expr, if any.
func wireFuncName(info *types.Info, expr ast.Expr) (string, bool) {
	var id *ast.Ident
	switch e := ast.Unparen(expr).(type) {
	case *ast.SelectorExpr:
		id = e.Sel
	case *ast.Ident: // dot import
		id = e
	default:
		return "", false
	}
	fn, ok := info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != WirePkgPath {
		return "", false
	}
	return fn.Name(), true
}

// setBuilder flattens wire.Build / wire.NewSet arguments into a set of
// providers and bindings.
type setBuilder struct {
	ctx       context.Context
	l         *Loader
	providers map[string]ProviderNode // keyed by ID; identical providers are deduplicated
	bindings  map[BindingSpec]struct{}
	visiting  map[string]bool // provider-set variables currently being expanded
	errs      []error
}

func newSetBuilder(ctx context.Context, l *Loader) *setBuilder {
	return &setBuilder{
		ctx:       ctx,
		l:         l,
		providers: make(map[string]ProviderNode),
		bindings:  make(map[BindingSpec]struct{}),
		visiting:  make(map[string]bool),
	}
}

func (b *setBuilder) errorf(pkg *packages.Package, p token.Pos, format string, args ...any) {
	b.errs = append(b.errs, fmt.Errorf("%s: "+format, append([]any{pkg.Fset.Position(p)}, args...)...))
}

// addExpr adds one wire.Build / wire.NewSet argument.
func (b *setBuilder) addExpr(pkg *packages.Package, expr ast.Expr) {
	expr = ast.Unparen(expr)
	info := pkg.TypesInfo

	if call, ok := expr.(*ast.CallExpr); ok {
		name, isWire := wireFuncName(info, call.Fun)
		switch {
		case !isWire:
			b.errorf(pkg, expr.Pos(), "%w: only provider functions, provider sets, wire.NewSet and wire.Bind are allowed, got call %s",
				ErrUnsupported, types.ExprString(expr))
		case name == "NewSet":
			for _, arg := range call.Args {
				b.addExpr(pkg, arg)
			}
		case name == "Bind":
			b.addBind(pkg, call)
		default:
			hint, known := unsupportedWireFuncs[name]
			if !known {
				hint = "it is not part of knit's supported subset"
			}
			b.errorf(pkg, expr.Pos(), "%w: wire.%s is not supported by knit; %s", ErrUnsupported, name, hint)
		}
		return
	}

	var id *ast.Ident
	switch e := expr.(type) {
	case *ast.Ident:
		id = e
	case *ast.SelectorExpr:
		id = e.Sel
	default:
		b.errorf(pkg, expr.Pos(), "%w: unexpected expression %s", ErrUnsupported, types.ExprString(expr))
		return
	}

	switch obj := info.Uses[id].(type) {
	case *types.Func:
		p, err := newProviderNode(obj)
		if err != nil {
			b.errorf(pkg, expr.Pos(), "%w", err)
			return
		}
		b.providers[p.ID] = p
	case *types.Var:
		b.addSetVar(pkg, expr, obj)
	default:
		b.errorf(pkg, expr.Pos(), "%w: %s is neither a provider function nor a provider set", ErrUnsupported, types.ExprString(expr))
	}
}

// addSetVar expands a package-level provider set variable, e.g.
// `var Set = wire.NewSet(...)`, which may live in another package.
func (b *setBuilder) addSetVar(pkg *packages.Package, ref ast.Expr, v *types.Var) {
	if v.Pkg() == nil || v.Parent() != v.Pkg().Scope() {
		b.errorf(pkg, ref.Pos(), "%w: %s must be a package-level provider set variable", ErrUnsupported, types.ExprString(ref))
		return
	}
	key := v.Pkg().Path() + "." + v.Name()
	if b.visiting[key] {
		b.errorf(pkg, ref.Pos(), "provider set %s refers to itself", key)
		return
	}
	b.visiting[key] = true
	defer delete(b.visiting, key)

	declPkg, err := b.l.pkgWithSyntax(b.ctx, v.Pkg().Path())
	if err != nil {
		b.errorf(pkg, ref.Pos(), "loading provider set %s: %v", key, err)
		return
	}
	value := findVarValue(declPkg, v.Name())
	if value == nil {
		b.errorf(pkg, ref.Pos(), "%w: provider set %s must be initialized with wire.NewSet(...)", ErrUnsupported, key)
		return
	}
	b.addExpr(declPkg, value)
}

// findVarValue returns the initializer expression of the package-level
// variable name in pkg, or nil.
func findVarValue(pkg *packages.Package, name string) ast.Expr {
	for _, f := range pkg.Syntax {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if n.Name == name && i < len(vs.Values) && len(vs.Names) == len(vs.Values) {
						return vs.Values[i]
					}
				}
			}
		}
	}
	return nil
}

// addBind handles wire.Bind(new(Iface), new(Impl)).
func (b *setBuilder) addBind(pkg *packages.Package, call *ast.CallExpr) {
	if len(call.Args) != 2 {
		b.errorf(pkg, call.Pos(), "%w: expected 2 arguments, got %d", ErrInvalidBind, len(call.Args))
		return
	}
	elem := func(arg ast.Expr) types.Type {
		ptr, ok := types.Unalias(pkg.TypesInfo.TypeOf(arg)).(*types.Pointer)
		if !ok {
			return nil
		}
		return ptr.Elem()
	}
	iface, impl := elem(call.Args[0]), elem(call.Args[1])
	if iface == nil || impl == nil {
		b.errorf(pkg, call.Pos(), "%w: arguments must be of the form new(Iface), new(Impl)", ErrInvalidBind)
		return
	}
	it, ok := iface.Underlying().(*types.Interface)
	if !ok {
		b.errorf(pkg, call.Pos(), "%w: first argument %s is not an interface type", ErrInvalidBind, NewTypeKey(iface))
		return
	}
	if !types.Implements(impl, it) {
		b.errorf(pkg, call.Pos(), "%w: %s does not implement %s", ErrInvalidBind, NewTypeKey(impl), NewTypeKey(iface))
		return
	}
	b.bindings[BindingSpec{InterfaceType: NewTypeKey(iface), ConcreteType: NewTypeKey(impl)}] = struct{}{}
}

func (b *setBuilder) sortedProviders() []ProviderNode {
	out := make([]ProviderNode, 0, len(b.providers))
	for _, p := range b.providers {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (b *setBuilder) sortedBindings() []BindingSpec {
	out := make([]BindingSpec, 0, len(b.bindings))
	for bs := range b.bindings {
		out = append(out, bs)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].InterfaceType != out[j].InterfaceType {
			return out[i].InterfaceType < out[j].InterfaceType
		}
		return out[i].ConcreteType < out[j].ConcreteType
	})
	return out
}

// newProviderNode validates fn as a provider and converts it to a ProviderNode.
func newProviderNode(fn *types.Func) (ProviderNode, error) {
	id := fn.FullName()
	sig := fn.Signature()
	if sig.Recv() != nil {
		return ProviderNode{}, fmt.Errorf("provider %s: %w: methods cannot be providers", id, ErrUnsupported)
	}
	if sig.TypeParams().Len() > 0 {
		return ProviderNode{}, fmt.Errorf("provider %s: %w: generic providers", id, ErrUnsupported)
	}
	params, err := paramSpecs(sig)
	if err != nil {
		return ProviderNode{}, fmt.Errorf("provider %s: %w", id, err)
	}
	out, hasCleanup, hasErr, err := classifyResults(sig.Results())
	if err != nil {
		return ProviderNode{}, fmt.Errorf("provider %s: %w", id, err)
	}
	return ProviderNode{
		ID:         id,
		PkgPath:    fn.Pkg().Path(),
		PkgName:    fn.Pkg().Name(),
		FuncName:   fn.Name(),
		Params:     params,
		OutputType: NewTypeKey(out),
		HasCleanup: hasCleanup,
		HasError:   hasErr,
	}, nil
}

// paramSpecs converts a signature's parameters, rejecting variadic ones.
func paramSpecs(sig *types.Signature) ([]ParamSpec, error) {
	ps := sig.Params()
	if sig.Variadic() {
		last := ps.At(ps.Len() - 1)
		elem := last.Type().(*types.Slice).Elem()
		name := last.Name()
		if name == "" {
			name = "_"
		}
		return nil, fmt.Errorf("%w: parameter %s ...%s; replace it with an explicit slice type []%s or a dedicated provider",
			ErrVariadic, name, NewTypeKey(elem).Short(), NewTypeKey(elem).Short())
	}
	out := make([]ParamSpec, 0, ps.Len())
	for i := 0; i < ps.Len(); i++ {
		p := ps.At(i)
		out = append(out, ParamSpec{Name: p.Name(), Type: NewTypeKey(p.Type())})
	}
	return out, nil
}

var (
	errorType   = types.Universe.Lookup("error").Type()
	cleanupType = types.NewSignatureType(nil, nil, nil, nil, nil, false)
)

func isError(t types.Type) bool   { return types.Identical(t, errorType) }
func isCleanup(t types.Type) bool { return types.Identical(t, cleanupType) }

// classifyResults validates a result tuple against the four supported shapes:
// (T), (T, error), (T, func()), (T, func(), error).
func classifyResults(res *types.Tuple) (out types.Type, hasCleanup, hasErr bool, err error) {
	bad := func() (types.Type, bool, bool, error) {
		return nil, false, false, fmt.Errorf("%w: results must be (T), (T, error), (T, func()) or (T, func(), error), got %s",
			ErrInvalidSignature, res.String())
	}
	switch res.Len() {
	case 1:
		out = res.At(0).Type()
	case 2:
		out = res.At(0).Type()
		switch second := res.At(1).Type(); {
		case isError(second):
			hasErr = true
		case isCleanup(second):
			hasCleanup = true
		default:
			return bad()
		}
	case 3:
		out = res.At(0).Type()
		if !isCleanup(res.At(1).Type()) || !isError(res.At(2).Type()) {
			return bad()
		}
		hasCleanup, hasErr = true, true
	default:
		return bad()
	}
	if isError(out) || isCleanup(out) {
		return bad()
	}
	return out, hasCleanup, hasErr, nil
}
