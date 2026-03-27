package type_constraint_union

type MyInt int
type Numeric interface{ MyInt | ~float64 }
type Container[T Numeric] struct{ val T }

func MockFunction() { print("ok") }
