package tio

type IdProvider interface {
	ID() (string, error)
}
