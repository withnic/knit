// Package lifo exercises cleanup ordering when a provider fails midway.
package lifo

import "errors"

// Log records resource lifecycle events in order.
var Log []string

// FailAt names the provider that returns an error ("" for none), NilCleanupAt
// the one that returns a nil cleanup and PanicAt the one whose cleanup panics.
var FailAt, NilCleanupAt, PanicAt string

type (
	A    struct{}
	B    struct{}
	C    struct{}
	Root struct{ C *C }
)

func open(name string) (func(), error) {
	if FailAt == name {
		return nil, errors.New("open " + name + " failed")
	}
	Log = append(Log, "open "+name)
	if NilCleanupAt == name {
		return nil, nil
	}
	return func() {
		Log = append(Log, "close "+name)
		if PanicAt == name {
			panic("close " + name + " failed")
		}
	}, nil
}

func NewA() (*A, func(), error) {
	c, err := open("A")
	if err != nil {
		return nil, nil, err
	}
	return &A{}, c, nil
}

func NewB(_ *A) (*B, func(), error) {
	c, err := open("B")
	if err != nil {
		return nil, nil, err
	}
	return &B{}, c, nil
}

func NewC(_ *B) (*C, func(), error) {
	c, err := open("C")
	if err != nil {
		return nil, nil, err
	}
	return &C{}, c, nil
}

func NewRoot(c *C) *Root { return &Root{C: c} }
