package type_func_field

type Foo struct {
	handler func(interface{ Bar() })
}

func MockFunction() { print("ok") }
