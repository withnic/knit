// Package app exercises import alias and local variable collisions.
package app

import (
	"context"

	one "example.com/fixture/gen/collide/one/config"
	two "example.com/fixture/gen/collide/two/config"
	v1 "example.com/fixture/gen/collide/v1"
)

// App is returned by value, so the error path needs an App{} zero value.
type App struct {
	One   *one.Config
	Two   *two.Config
	Thing *v1.Thing
	Label string
}

// cleanup is a package-level name the generated code must not shadow.
func cleanup() {}

var _ = cleanup

func NewApp(_ context.Context, a *one.Config, b *two.Config, t *v1.Thing, label string) (App, error) {
	return App{One: a, Two: b, Thing: t, Label: label}, nil
}
