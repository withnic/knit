package v1

type Thing struct{}

func NewThing() (*Thing, func()) { return &Thing{}, func() {} }
