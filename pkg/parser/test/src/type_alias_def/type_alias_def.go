package type_alias_def

type Original struct{}
type Alias = Original
type NewType Original

func MockFunction() { print("ok") }
