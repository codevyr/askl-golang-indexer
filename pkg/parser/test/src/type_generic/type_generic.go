package type_generic

type Stringer interface{ String() string }
type Collection[T Stringer] struct{ items []T }

func MockFunction() { print("ok") }
