// Package duplicate supplies an interface twice, deep in the dependency graph.
package duplicate

type Store interface{ Get() string }

type (
	SQLStore struct{}
	Repo     struct{}
	Service  struct{}
)

func (*SQLStore) Get() string { return "" }

func NewSQLStore() *SQLStore { return &SQLStore{} }

func NewStore() Store { return &SQLStore{} }

func NewRepo(Store) *Repo { return &Repo{} }

func NewService(*Repo) *Service { return &Service{} }
