package return_values

type wrapErrors int
type wrapErrorsAlias = wrapErrors
type wrapErrorsAliasAlias = wrapErrorsAlias

func (e *wrapErrors) Error() string {
	return "wrapped error"
}

func (e *wrapErrors) Unwrap() []error {
	return nil
}

type Mock interface {
	MockFunction() error
}

func Foo() (Mock, error) {
	var m Mock
	err := wrapErrorsAliasAlias(0)
	return m, &err
}

func MockFunction() {
	_, err := Foo()
	if err != nil {
		print("Hello, World!")
	}
}
