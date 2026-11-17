package di

import (
	"fmt"
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
func (e *ResolveError) Error() string {
	parts := make([]string, len(e.Chain))
	for i, k := range e.Chain {
		parts[i] = k.Short()
	}
	reason := e.Kind.String()
	if e.Detail != "" {
		reason += ": " + e.Detail
	}
	return fmt.Sprintf("failed to resolve provider for injector '%s':\n  %s (%s)",
		e.Injector, strings.Join(parts, " -> "), reason)
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
	// BoundFrom is the concrete type the value was bound from via wire.Bind,
	// or empty when Type was provided directly.
	BoundFrom TypeKey
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
	// needed collects the provider output types required by the root.
	needed map[TypeKey]bool
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
// The order is deterministic: Kahn's algorithm is used and, among providers
// that are ready at the same time, the one with the smallest ID comes first.
// Missing, duplicate and circular dependencies are reported as *ResolveError
// with the full dependency chain.
func Resolve(inj *InjectorSpec) (*ResolvedInjector, error) {
	r := &resolver{
		inj:       inj,
		params:    make(map[TypeKey]int, len(inj.Params)),
		providers: make(map[TypeKey]*ProviderNode, len(inj.Providers)),
		bindings:  make(map[TypeKey]TypeKey, len(inj.Bindings)),
		state:     make(map[TypeKey]visitState),
		needed:    make(map[TypeKey]bool),
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

// buildTables indexes params, providers and bindings, reporting any type that
// is supplied by more than one source.
func (r *resolver) buildTables() error {
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
		sources[b.InterfaceType] = append(sources[b.InterfaceType], "wire.Bind to "+b.ConcreteType.String())
	}

	keys := make([]TypeKey, 0, len(sources))
	for t := range sources {
		keys = append(keys, t)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, t := range keys {
		if s := sources[t]; len(s) > 1 {
			sort.Strings(s)
			return r.errorf(ErrKindDuplicateProvider, []TypeKey{t}, strings.Join(s, ", "))
		}
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
	r.needed[t] = true

	if p.HasError && !r.inj.HasError {
		return r.errorf(ErrKindMissingError, r.stack, "provider "+p.ID+" returns an error")
	}
	if p.HasCleanup && !r.inj.HasCleanup {
		return r.errorf(ErrKindMissingCleanup, r.stack, "provider "+p.ID+" returns a cleanup function")
	}
	return nil
}

// source returns the provider output type or parameter that ultimately
// supplies t after following bindings, plus the bound concrete type.
func (r *resolver) source(t TypeKey) (supplier TypeKey, boundFrom TypeKey) {
	supplier = t
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
	if supplier != t {
		boundFrom = supplier
	}
	return supplier, boundFrom
}

// plan orders the needed providers with Kahn's algorithm and wires up the
// argument references.
func (r *resolver) plan() (*ResolvedInjector, error) {
	// Deterministic node list.
	nodes := make([]TypeKey, 0, len(r.needed))
	for t := range r.needed {
		nodes = append(nodes, t)
	}
	sort.Slice(nodes, func(i, j int) bool { return r.providers[nodes[i]].ID < r.providers[nodes[j]].ID })

	indeg := make(map[TypeKey]int, len(nodes))
	dependents := make(map[TypeKey][]TypeKey, len(nodes))
	for _, t := range nodes {
		deps := make(map[TypeKey]bool)
		for _, param := range r.providers[t].Params {
			sup, _ := r.source(param.Type)
			if r.needed[sup] && !deps[sup] {
				deps[sup] = true
				indeg[t]++
				dependents[sup] = append(dependents[sup], t)
			}
		}
	}

	var ready []TypeKey
	for _, t := range nodes {
		if indeg[t] == 0 {
			ready = append(ready, t)
		}
	}
	byID := func(s []TypeKey) {
		sort.Slice(s, func(i, j int) bool { return r.providers[s[i]].ID < r.providers[s[j]].ID })
	}

	stepIndex := make(map[TypeKey]int, len(nodes))
	res := &ResolvedInjector{Injector: r.inj}
	for len(ready) > 0 {
		byID(ready)
		t := ready[0]
		ready = ready[1:]

		p := r.providers[t]
		step := Step{Provider: *p, Args: make([]ValueRef, len(p.Params))}
		for i, param := range p.Params {
			step.Args[i] = r.ref(param.Type, stepIndex)
		}
		stepIndex[t] = len(res.Steps)
		res.Steps = append(res.Steps, step)

		for _, d := range dependents[t] {
			indeg[d]--
			if indeg[d] == 0 {
				ready = append(ready, d)
			}
		}
	}
	if len(res.Steps) != len(nodes) {
		// Unreachable after a successful DFS, kept as a safety net.
		var rest []TypeKey
		for _, t := range nodes {
			if _, ok := stepIndex[t]; !ok {
				rest = append(rest, t)
			}
		}
		return nil, r.errorf(ErrKindCircularDependency, rest, "topological sort did not converge")
	}
	res.Root = r.ref(r.inj.ReturnType, stepIndex)
	return res, nil
}

func (r *resolver) ref(t TypeKey, stepIndex map[TypeKey]int) ValueRef {
	sup, bound := r.source(t)
	if i, ok := r.params[sup]; ok {
		return ValueRef{Type: t, Kind: FromParam, Index: i, BoundFrom: bound}
	}
	return ValueRef{Type: t, Kind: FromProvider, Index: stepIndex[sup], BoundFrom: bound}
}
