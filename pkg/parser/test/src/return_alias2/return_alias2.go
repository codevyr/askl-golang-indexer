package return_alias2

type Mock interface {
	MockFunction() error
}

type MockImpl struct {
}

func (m MockImpl) MockFunction() error {
	return nil
}

func Foo(v any) any {
	switch v := v.(type) {
	case Mock:
		return v
	default:
		return nil
	}
}

func MockFunction() {
	v := Foo(&MockImpl{})
	if v != nil {
		print("Hello, World!")
	}
}
