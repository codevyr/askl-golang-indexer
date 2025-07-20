package return_values2

type Mock interface {
	MockFunction() error
}

func Foo() (Mock, error) {
	var m Mock
	return m, nil
}

func MockFunction() {
	_, err := Foo()
	if err != nil {
		print("Hello, World!")
	}
}
