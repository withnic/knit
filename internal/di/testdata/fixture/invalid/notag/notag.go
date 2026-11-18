// Package notag declares an injector in a file without the inject build tag.
package notag

type T struct{}

func NewT() *T { return &T{} }
