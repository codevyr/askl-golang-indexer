package type_nested_expr

type Key struct{}
type Value struct{}
type Complex map[*Key][]Value

func MockFunction() { print("ok") }
