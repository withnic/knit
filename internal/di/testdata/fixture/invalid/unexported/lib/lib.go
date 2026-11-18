// Package lib exposes a provider set containing an unexported provider.
package lib

import "github.com/google/wire"

type T struct{}

func newT() *T { return &T{} }

var Set = wire.NewSet(newT)
