// Package alias uses a type alias under a pointer, in a slice and as a type
// argument.
package alias

type (
	Config      struct{}
	ConfigAlias = Config
	Box[T any]  struct{}
	Root        struct{}
)

func NewConfig() *ConfigAlias { return &Config{} }

func NewConfigs() []ConfigAlias { return nil }

func NewBox() Box[ConfigAlias] { return Box[ConfigAlias]{} }

func NewRoot(*Config, []Config, Box[Config]) *Root { return &Root{} }
