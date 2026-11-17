// Package di implements knit's dependency injection code generation engine.
//
// It loads injector declarations written with a marker package (knit's own
// github.com/withnic/knit by default, or github.com/google/wire in wire mode;
// see Mode), resolves their dependency graphs and renders the generated code
// and the di-graph.json document.
package di

import (
	"go/types"
	"regexp"
	"strings"
)

// TypeKey is a fully qualified, normalized type name that uniquely identifies
// a Go type across packages.
//
// Examples:
//
//	github.com/org/repo/internal/service.UserService
//	*github.com/org/repo/internal/user.ID
//
// Normalization rules (see docs/02_data_models.md §1.4):
//   - pointer and value types are distinct keys (the leading "*" is kept);
//   - named types are qualified by their full import path, so same-named types
//     in different packages never collide;
//   - variadic parameters (...T) are never converted into a TypeKey; they are
//     rejected with a validation error by the loader.
type TypeKey string

// String returns the TypeKey as a plain string.
func (k TypeKey) String() string { return string(k) }

// IsPointer reports whether the key denotes a pointer type.
func (k TypeKey) IsPointer() bool { return strings.HasPrefix(string(k), "*") }

// pkgPathPrefix matches the import-path portion ("github.com/org/repo/internal/")
// preceding a package-qualified identifier inside a TypeKey.
var pkgPathPrefix = regexp.MustCompile(`(?:[A-Za-z0-9_.\-~]+/)+`)

// Short returns a human-readable form of the key in which every import path is
// reduced to its package name, e.g. "*github.com/org/repo/db.Pool" -> "*db.Pool".
// It is intended for error messages only and is not guaranteed to be unique.
func (k TypeKey) Short() string {
	return pkgPathPrefix.ReplaceAllString(string(k), "")
}

// NewTypeKey builds the normalized TypeKey for t.
//
// Top-level type aliases are resolved to their target so that an alias and its
// aliased type share the same key.
func NewTypeKey(t types.Type) TypeKey {
	t = types.Unalias(t)
	return TypeKey(types.TypeString(t, func(p *types.Package) string { return p.Path() }))
}

// ParamSpec describes one parameter of a provider or injector function.
type ParamSpec struct {
	// Name is the parameter name in the source, used as a variable name hint.
	// It may be empty for unnamed parameters.
	Name string
	// Type is the normalized type of the parameter.
	Type TypeKey
}

// ProviderNode represents a single constructor function (provider).
type ProviderNode struct {
	// ID uniquely identifies the provider: "<PkgPath>.<FuncName>".
	ID string
	// PkgPath is the import path of the package declaring the provider.
	PkgPath string
	// PkgName is the package name of the package declaring the provider.
	PkgName string
	// FuncName is the provider function name.
	FuncName string
	// Params lists the provider's dependencies in declaration order.
	Params []ParamSpec
	// OutputType is the type produced by the provider (its first result).
	OutputType TypeKey
	// HasCleanup reports whether the second result is a func() cleanup.
	HasCleanup bool
	// HasError reports whether the last result is an error.
	HasError bool
}

// BindingSpec represents wire.Bind(new(Iface), new(Impl)): every request for
// InterfaceType is satisfied by the value of ConcreteType.
type BindingSpec struct {
	// InterfaceType is the fully qualified interface type.
	InterfaceType TypeKey
	// ConcreteType is the fully qualified implementation type (may be a pointer).
	ConcreteType TypeKey
}

// InjectorSpec represents an injector function, i.e. a function whose body
// calls wire.Build(...).
type InjectorSpec struct {
	// FuncName is the injector function name, e.g. InitializeUserHandler.
	FuncName string
	// PkgPath is the import path of the package the code is generated into.
	PkgPath string
	// PkgName is the package name of the package the code is generated into.
	PkgName string
	// FilePath is the path of the source file (wire.go) declaring the injector.
	FilePath string
	// Params are the values passed into the injector by its caller.
	Params []ParamSpec
	// ReturnType is the root type the injector builds.
	ReturnType TypeKey
	// HasCleanup reports whether the injector returns a func() cleanup.
	HasCleanup bool
	// HasError reports whether the injector returns an error.
	HasError bool
	// Providers is the flattened list of providers available to the injector,
	// sorted by ID.
	Providers []ProviderNode
	// Bindings is the flattened list of interface bindings, sorted by
	// InterfaceType.
	Bindings []BindingSpec

	// Func is the type-checked injector declaration. It is set by the Loader
	// and is required by the code generator to render the injector signature
	// and imports. It is nil for specs built by hand (e.g. resolver tests).
	Func *types.Func
}
