// Package shapes covers every supported result shape of providers and
// injectors.
package shapes

// Log records cleanup calls in order.
var Log []string

type (
	Value struct{}
	Err   struct{}
	Clean struct{}
	Both  struct{}
	Root  struct{}
)

func closer(name string) func() { return func() { Log = append(Log, name) } }

func NewValue() *Value { return &Value{} }

func NewErr(*Value) (*Err, error) { return &Err{}, nil }

func NewClean(*Value) (*Clean, func()) { return &Clean{}, closer("clean") }

func NewBoth(*Clean, *Err) (*Both, func(), error) { return &Both{}, closer("both"), nil }

func NewRoot(*Both) *Root { return &Root{} }
