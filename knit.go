// Package knit is the marker API for declaring injectors and provider sets
// for the knit code generator (https://github.com/withnic/knit).
//
// Injectors are declared in files guarded by the knitinject build tag. Their
// body is a single call to Build listing the providers, provider sets and
// bindings the injector may use; knit gen replaces it with real code in
// knit_gen.go:
//
//	//go:build knitinject
//
//	package di
//
//	import "github.com/withnic/knit"
//
//	func InitializeServer(ctx context.Context) (*Server, func(), error) {
//		panic(knit.Build(StoreSet, NewServer))
//	}
//
//	var StoreSet = knit.NewSet(NewSQLStore, knit.Bind(new(Store), new(*SQLStore)))
//
// The functions in this package only mark declarations for static analysis;
// they do nothing at run time.
package knit

// ProviderSet is a group of providers and bindings created by NewSet.
type ProviderSet struct{}

// NewSet groups providers (functions returning (T), (T, error), (T, func())
// or (T, func(), error)), other provider sets and bindings so they can be
// reused across injectors.
func NewSet(...any) ProviderSet { return ProviderSet{} }

// Build declares an injector. It must be the only statement of the injector
// body, typically as panic(knit.Build(...)), and lists the providers,
// provider sets and bindings available to it.
func Build(...any) string {
	return "knit: injector not generated; run knit gen"
}

// Binding binds an interface type to a concrete type.
type Binding struct{}

// Bind declares that every request for the interface pointed to by iface is
// satisfied by the concrete type pointed to by to:
//
//	knit.Bind(new(Store), new(*SQLStore))
func Bind(iface, to any) Binding { return Binding{} } //nolint:revive // names document the call shape; the body is a marker
