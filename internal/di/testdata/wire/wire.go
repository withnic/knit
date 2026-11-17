// Package wire is a minimal test stub of github.com/google/wire's marker API.
// Only the signatures matter: knit analyzes calls to these functions.
package wire

type ProviderSet struct{}

func NewSet(...interface{}) ProviderSet { return ProviderSet{} }

func Build(...interface{}) string { return "implementation not generated" }

type Binding struct{}

func Bind(iface, to interface{}) Binding { return Binding{} }

type StructProvider struct{}

func Struct(structType interface{}, fieldNames ...string) StructProvider { return StructProvider{} }

type ProvidedValue struct{}

func Value(interface{}) ProvidedValue { return ProvidedValue{} }

func InterfaceValue(typ interface{}, x interface{}) ProvidedValue { return ProvidedValue{} }

type StructFields struct{}

func FieldsOf(structType interface{}, fieldNames ...string) StructFields { return StructFields{} }
