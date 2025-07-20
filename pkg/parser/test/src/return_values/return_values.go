package return_values

type wrapErrors struct {
	msg string
}

func (e *wrapErrors) Error() string {
	return e.msg
}

func (e *wrapErrors) Unwrap() []error {
	return nil
}

type Mock interface {
	MockFunction() error
}

func Foo() (Mock, error) {
	var m Mock
	return m, &wrapErrors{}
}

func MockFunction() {
	_, err := Foo()
	if err != nil {
		print("Hello, World!")
	}
}
