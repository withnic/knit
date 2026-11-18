//go:build wireinject

package di

import (
	"example.com/fixture/invalid/unexported/lib"
	"github.com/google/wire"
)

func InitializeT() *lib.T {
	panic(wire.Build(lib.Set))
}
