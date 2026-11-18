//go:build wireinject

package tagged

import "github.com/google/wire"

func NewT() *T { return &T{} }

func InitializeT() *T {
	panic(wire.Build(NewT))
}
