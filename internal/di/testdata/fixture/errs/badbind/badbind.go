// Package badbind binds interfaces to types that do not implement them.
package badbind

type (
	Store  interface{ Get() string }
	Closer interface{ Close() error }
)

type (
	SQLStore struct{}
	Service  struct{}
	Other    struct{}
)

func (*SQLStore) Get() string { return "" }

func (Other) Close() {}

func NewSQLStore() SQLStore { return SQLStore{} }

func NewOther() Other { return Other{} }

func NewService(Store) *Service { return &Service{} }

func NewClosing(Closer) *Service { return &Service{} }
