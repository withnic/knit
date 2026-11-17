//go:build knitinject

package app

import (
	"context"

	"github.com/withnic/knit"
)

var StoreSet = knit.NewSet(NewDSN, NewSQLStore, knit.Bind(new(Store), new(*SQLStore)))

func InitializeServer(ctx context.Context) (*Server, func(), error) {
	panic(knit.Build(StoreSet, NewServer))
}
