//go:build wireinject

package app

import (
	v1 "example.com/fixture/gen/collide/v1"
	"github.com/google/wire"
)

func InitializeThing() (*v1.Thing, func()) {
	panic(wire.Build(v1.NewThing))
}
