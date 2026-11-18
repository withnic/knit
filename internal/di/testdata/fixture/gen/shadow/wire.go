//go:build wireinject

package shadow

import "github.com/google/wire"

func InitializeB() (*B, func(), error) {
	panic(wire.Build(NewA, NewB))
}
