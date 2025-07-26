package return_index_expression

type Token interface {
	GetText() string
}

type TokenImpl struct {
	text string
}

func (t TokenImpl) GetText() string {
	return t.text
}

func Foo() string {
	m := map[int]Token{}
	if _, ok := m[0]; ok {
		m[0] = TokenImpl{text: "Hello, World!"}
	}
	return ""
}

func MockFunction() {
	s := Foo()
	if s != "" {
		print("Hello, World!")
	}
}
