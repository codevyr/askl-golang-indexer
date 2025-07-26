package return_type_assert

type Token interface {
	GetText() string
}

func Foo() string {
	payload := interface{}(nil)
	if p2, ok := payload.(Token); ok {
		return p2.GetText()
	}
	return ""
}

func MockFunction() {
	s := Foo()
	if s != "" {
		print("Hello, World!")
	}
}
