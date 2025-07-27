package return_grouped

type Mock interface {
	MockFunction() error
}

func Foo() (a, b int, err error) {
	return 0, 1, nil
}

func MockFunction() {
	a, b, err := Foo()
	if err != nil {
		print("Hello, World!", a, b)
	}
}
