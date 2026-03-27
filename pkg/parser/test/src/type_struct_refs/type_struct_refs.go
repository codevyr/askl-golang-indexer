package type_struct_refs

type Bar struct{}
type Baz struct{}
type Foo struct {
	B     *Bar
	Items []Baz
}

func MockFunction() { print("ok") }
