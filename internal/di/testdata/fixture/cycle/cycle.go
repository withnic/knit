package cycle

type A struct{}
type B struct{}
type C struct{}

func NewA(b *B) *A { return &A{} }
func NewB(c *C) *B { return &B{} }
func NewC(a *A) *C { return &C{} }
