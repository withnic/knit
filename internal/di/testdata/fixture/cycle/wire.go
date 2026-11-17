//go:build wireinject

package cycle

import "github.com/google/wire"

func InitializeA() *A {
	panic(wire.Build(NewA, NewB, NewC))
}
