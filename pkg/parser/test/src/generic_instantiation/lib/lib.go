package generic_instantiation_lib

type Box[T any] struct{}

func (b Box[T]) Foo() {}
