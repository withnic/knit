//go:build wireinject

package lifo

import "github.com/google/wire"

func InitializeRoot() (*Root, func(), error) {
	panic(wire.Build(NewA, NewB, NewC, NewRoot))
}
