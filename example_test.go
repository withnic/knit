package knit_test

import (
	"github.com/withnic/knit"
)

// Providers are ordinary constructors. They may also return a cleanup
// function and/or an error.

type Store interface{ Get() string }

type SQLStore struct{}

func (*SQLStore) Get() string { return "value" }

func NewSQLStore() (*SQLStore, func(), error) {
	return &SQLStore{}, func() { /* close the connection */ }, nil
}

type Server struct{ Store Store }

func NewServer(s Store) *Server { return &Server{Store: s} }

// StoreSet groups providers and bindings for reuse across injectors.
var StoreSet = knit.NewSet(NewSQLStore, knit.Bind(new(Store), new(*SQLStore)))

// InitializeServer is an injector. In a real project it lives in a file
// guarded by //go:build knitinject, and `knit gen` writes its implementation
// to knit_gen.go.
func InitializeServer() (*Server, func(), error) {
	panic(knit.Build(StoreSet, NewServer))
}

func Example() {
	// After `knit gen`, the generated InitializeServer is a regular function:
	//
	//	srv, cleanup, err := InitializeServer()
	//	if err != nil {
	//		return err
	//	}
	//	defer cleanup()
	_ = InitializeServer
}
