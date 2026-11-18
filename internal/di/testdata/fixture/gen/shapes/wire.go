//go:build wireinject

package shapes

import "github.com/google/wire"

func InitializeValue() *Value {
	panic(wire.Build(NewValue))
}

func InitializeErr() (*Err, error) {
	panic(wire.Build(NewValue, NewErr))
}

func InitializeClean() (*Clean, func()) {
	panic(wire.Build(NewValue, NewClean))
}

func InitializeRoot() (*Root, func(), error) {
	panic(wire.Build(NewValue, NewErr, NewClean, NewBoth, NewRoot))
}

// InitializeNoCleanup returns a cleanup although no provider has one.
func InitializeNoCleanup() (*Value, func(), error) {
	panic(wire.Build(NewValue))
}
