//go:build wireinject

package app

import (
	"context"

	one "example.com/fixture/gen/collide/one/config"
	two "example.com/fixture/gen/collide/two/config"
	v1 "example.com/fixture/gen/collide/v1"
	"github.com/google/wire"
)

// The parameter named "config" collides with both config packages.
func InitializeApp(ctx context.Context, config string) (App, func(), error) {
	panic(wire.Build(one.New, two.New, v1.NewThing, NewApp))
}
