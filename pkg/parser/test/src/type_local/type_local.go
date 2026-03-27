package type_local

type Foo struct{}

func Outer() {
	type Foo struct{ X int }
	print("ok")
}

func MockFunction() { print("ok") }
