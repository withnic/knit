package notag

import "github.com/google/wire"

func InitializeT() *T {
	panic(wire.Build(NewT))
}
