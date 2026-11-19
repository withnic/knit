package di

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// ResolveErrorKind classifies dependency resolution failures.
type ResolveErrorKind int

const (
	// ErrKindMissingProvider means no provider, binding or injector parameter
	// supplies a required type.
	ErrKindMissingProvider ResolveErrorKind = iota + 1
	// ErrKindDuplicateProvider means a type is supplied more than once.
	ErrKindDuplicateProvider
	// ErrKindCircularDependency means the dependency graph contains a cycle.
	ErrKindCircularDependency
	// ErrKindMissingCleanup means a provider returns a cleanup function but the
	// injector does not.
	ErrKindMissingCleanup
	// ErrKindMissingError means a provider returns an error but the injector
	// does not.
	ErrKindMissingError
	// ErrKindInvalidBind means a binding's concrete type does not implement
	// its interface.
	ErrKindInvalidBind
)

func (k ResolveErrorKind) String() string {
	switch k {
	case ErrKindMissingProvider:
		return "provider not found"
	case ErrKindDuplicateProvider:
		return "duplicate provider"
	case ErrKindCircularDependency:
		return "circular dependency"
	case ErrKindMissingCleanup:
		return "injector must return a cleanup function"
	case ErrKindMissingError:
		return "injector must return an error"
	case ErrKindInvalidBind:
		return "invalid Bind"
	default:
		return "unknown resolve error"
	}
}

// ResolveError is a dependency resolution failure carrying the full dependency
// chain from the injector's root type to the offending type.
type ResolveError struct {
	// Injector is the name of the injector being resolved.
	Injector string
	// Kind classifies the failure.
	Kind ResolveErrorKind
	// Chain is the dependency path from the root type to the failing type.
	// For circular dependencies the last element repeats the first element of
	// the cycle.
	Chain []TypeKey
	// Detail optionally adds context (e.g. the conflicting provider IDs).
	Detail string
}

// Error renders the error, e.g.:
//
//	failed to resolve provider for injector 'InitializeUserService':
//	  service.UserService -> repo.UserRepository -> *db.Pool (provider not found)
//
// Types whose short names are ambiguous within the chain are shown in full.
func (e *ResolveError) Error() string {
	owner := make(map[string]TypeKey, len(e.Chain))
	ambiguous := make(map[string]bool)
	for _, k := range e.Chain {
		s := k.Short()
		if o, seen := owner[s]; seen && o != k {
			ambiguous[s] = true
		}
		owner[s] = k
	}
	parts := make([]string, len(e.Chain))
	for i, k := range e.Chain {
		parts[i] = k.Short()
		if ambiguous[parts[i]] {
			parts[i] = k.String()
		}
	}
	reason := e.Kind.String()
	if e.Detail != "" {
		reason += ": " + e.Detail
	}
	return fmt.Sprintf("failed to resolve provider for injector '%s':\n  %s (%s)",
		e.Injector, strings.Join(parts, " -> "), reason)
}

// Unwrap lets errors.Is match ErrInvalidBind for invalid bindings.
func (e *ResolveError) Unwrap() error {
	if e.Kind == ErrKindInvalidBind {
		return ErrInvalidBind
	}
	return nil
}

// SourceKind tells where a value consumed by the injector comes from.
type SourceKind int

const (
	// FromParam means the value is an injector parameter.
	FromParam SourceKind = iota + 1
	// FromProvider means the value is the output of a provider step.
	FromProvider
)

// ValueRef identifies the value that satisfies a requested type.
type ValueRef struct {
	// Type is the requested type (e.g. an interface).
	Type TypeKey
	// Kind is where the value comes from.
	Kind SourceKind
	// Index is the injector parameter index (FromParam) or the step index in
	// ResolvedInjector.Steps (FromProvider).
	Index int
}

// Step is one provider call in dependency order.
type Step struct {
	// Provider is the provider to call.
	Provider ProviderNode
	// Args are the values passed to the provider, one per parameter.
	Args []ValueRef
}

// ResolvedInjector is the dependency plan of an injector: provider calls in a
// valid topological order and the value returned as the injector's result.
type ResolvedInjector struct {
	Injector *InjectorSpec
	// Steps lists the providers to call; every step only depends on injector
	// parameters and earlier steps.
	Steps []Step
	// Root is the value returned by the injector.
	Root ValueRef
}

// resolver holds the lookup tables for one injector.
type resolver struct {
	inj       *InjectorSpec
	params    map[TypeKey]int           // injector parameter type -> index
	providers map[TypeKey]*ProviderNode // output type -> provider
	bindings  map[TypeKey]TypeKey       // interface -> concrete

	// DFS state.
	state map[TypeKey]visitState
	stack []TypeKey
	// order collects the provider output types required by the root in
	// depth-first post-order.
	order []TypeKey
}

type visitState int

const (
	unvisited visitState = iota
	visiting
	done
)

// Resolve builds the dependency graph of inj starting from its return type and
// returns the providers in topological order.
//
// The order matches google/wire: a depth-first walk from the return type that
// visits each provider's parameters in declaration order and calls a provider
// once all of them are built. It depends only on the dependency graph and the
// parameter order, not on provider names or the order of the provider sets.
// Missing, duplicate and circular dependencies are reported as *ResolveError
// with the full dependency chain.
func Resolve(inj *InjectorSpec) (*ResolvedInjector, error) {
	r := &resolver{
		inj:       inj,
		params:    make(map[TypeKey]int, len(inj.Params)),
		providers: make(map[TypeKey]*ProviderNode, len(inj.Providers)),
		bindings:  make(map[TypeKey]TypeKey, len(inj.Bindings)),
		state:     make(map[TypeKey]visitState),
	}
	if err := r.buildTables(); err != nil {
		return nil, err
	}
	if err := r.visit(inj.ReturnType); err != nil {
		return nil, err
	}
	return r.plan()
}

func (r *resolver) errorf(kind ResolveErrorKind, chain []TypeKey, detail string) *ResolveError {
	return &ResolveError{
		Injector: r.inj.FuncName,
		Kind:     kind,
		Chain:    append([]TypeKey(nil), chain...),
		Detail:   detail,
	}
}

// pathTo returns a shortest dependency path from the injector's root type to
// target, considering every provider and binding, or nil if target is not
// reachable. It works on the raw declarations, so it can explain conflicts
// found before the lookup tables are usable.
func (r *resolver) pathTo(target TypeKey) []TypeKey {
	deps := make(map[TypeKey][]TypeKey)
	for i := range r.inj.Providers {
		p := &r.inj.Providers[i]
		for _, param := range p.Params {
			deps[p.OutputType] = append(deps[p.OutputType], param.Type)
		}
	}
	for _, b := range r.inj.Bindings {
		deps[b.InterfaceType] = append(deps[b.InterfaceType], b.ConcreteType)
	}
	root := r.inj.ReturnType
	parent := map[TypeKey]TypeKey{root: ""}
	for queue := []TypeKey{root}; len(queue) > 0; queue = queue[1:] {
		t := queue[0]
		if t == target {
			var path []TypeKey
			for ; t != ""; t = parent[t] {
				path = append(path, t)
			}
			slices.Reverse(path)
			return path
		}
		next := deps[t]
		sort.Slice(next, func(i, j int) bool { return next[i] < next[j] })
		for _, d := range next {
			if _, seen := parent[d]; !seen {
				parent[d] = t
				queue = append(queue, d)
			}
		}
	}
	return nil
}

// buildTables indexes params, providers and bindings, reporting invalid
// bindings and any type that is supplied by more than one source.
func (r *resolver) buildTables() error {
	for _, b := range r.inj.Bindings {
		if b.Problem == "" {
			continue
		}
		chain := r.pathTo(b.InterfaceType)
		if chain == nil {
			chain = []TypeKey{b.InterfaceType}
		}
		return r.errorf(ErrKindInvalidBind, append(chain, b.ConcreteType), b.Problem)
	}

	sources := make(map[TypeKey][]string)
	for i, p := range r.inj.Params {
		r.params[p.Type] = i
		name := p.Name
		if name == "" {
			name = fmt.Sprintf("#%d", i)
		}
		sources[p.Type] = append(sources[p.Type], "injector parameter "+name)
	}
	for i := range r.inj.Providers {
		p := &r.inj.Providers[i]
		r.providers[p.OutputType] = p
		sources[p.OutputType] = append(sources[p.OutputType], p.ID)
	}
	for _, b := range r.inj.Bindings {
		r.bindings[b.InterfaceType] = b.ConcreteType
		sources[b.InterfaceType] = append(sources[b.InterfaceType], "Bind to "+b.ConcreteType.String())
	}

	keys := make([]TypeKey, 0, len(sources))
	for t := range sources {
		keys = append(keys, t)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, t := range keys {
		s := sources[t]
		if len(s) < 2 {
			continue
		}
		sort.Strings(s)
		detail := strings.Join(s, ", ")
		chain := r.pathTo(t)
		if chain == nil {
			chain = []TypeKey{t}
			detail += "; not needed by " + r.inj.ReturnType.Short()
		}
		return r.errorf(ErrKindDuplicateProvider, chain, detail)
	}
	return nil
}

// visit walks the dependency graph depth-first from t, detecting missing
// providers and cycles while the full chain is still on the stack.
func (r *resolver) visit(t TypeKey) error {
	r.stack = append(r.stack, t)
	defer func() { r.stack = r.stack[:len(r.stack)-1] }()

	if _, ok := r.params[t]; ok {
		return nil
	}
	switch r.state[t] {
	case done:
		return nil
	case visiting:
		// The chain is the path from the root, ending with the repeated type.
		return r.errorf(ErrKindCircularDependency, r.stack, "")
	}
	if concrete, ok := r.bindings[t]; ok {
		r.state[t] = visiting
		if err := r.visit(concrete); err != nil {
			return err
		}
		r.state[t] = done
		return nil
	}
	p, ok := r.providers[t]
	if !ok {
		return r.errorf(ErrKindMissingProvider, r.stack, "")
	}
	r.state[t] = visiting
	for _, param := range p.Params {
		if err := r.visit(param.Type); err != nil {
			return err
		}
	}
	r.state[t] = done
	r.order = append(r.order, t)

	if p.HasError && !r.inj.HasError {
		return r.errorf(ErrKindMissingError, r.stack, "provider "+p.ID+" returns an error")
	}
	if p.HasCleanup && !r.inj.HasCleanup {
		return r.errorf(ErrKindMissingCleanup, r.stack, "provider "+p.ID+" returns a cleanup function")
	}
	return nil
}

// source returns the provider output type or parameter that ultimately
// supplies t after following bindings.
func (r *resolver) source(t TypeKey) TypeKey {
	supplier := t
	for {
		if _, ok := r.params[supplier]; ok {
			break
		}
		c, ok := r.bindings[supplier]
		if !ok {
			break
		}
		supplier = c
	}
	return supplier
}

// plan turns the providers collected by visit into steps and wires up the
// argument references.
func (r *resolver) plan() (*ResolvedInjector, error) {
	stepIndex := make(map[TypeKey]int, len(r.order))
	res := &ResolvedInjector{Injector: r.inj, Steps: make([]Step, 0, len(r.order))}
	for _, t := range r.order {
		p := r.providers[t]
		step := Step{Provider: *p, Args: make([]ValueRef, len(p.Params))}
		for i, param := range p.Params {
			step.Args[i] = r.ref(param.Type, stepIndex)
		}
		stepIndex[t] = len(res.Steps)
		res.Steps = append(res.Steps, step)
	}
	res.Root = r.ref(r.inj.ReturnType, stepIndex)
	return res, nil
}

func (r *resolver) ref(t TypeKey, stepIndex map[TypeKey]int) ValueRef {
	sup := r.source(t)
	if i, ok := r.params[sup]; ok {
		return ValueRef{Type: t, Kind: FromParam, Index: i}
	}
	return ValueRef{Type: t, Kind: FromProvider, Index: stepIndex[sup]}
}
