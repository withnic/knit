// Package shadow redeclares builtins at package level; generated cleanup code
// must not depend on them.
package shadow

type (
	A struct{}
	B struct{}
)

func len(string) int { return 0 }

func append() {}

var _, _ = len, append

func NewA() (*A, func()) { return &A{}, func() {} }

func NewB(*A) (*B, func(), error) { return &B{}, func() {}, nil }
