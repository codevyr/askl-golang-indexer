package type_embedded

type Base struct{}
type Extended struct {
	Base
	X int
}

func MockFunction() { print("ok") }
