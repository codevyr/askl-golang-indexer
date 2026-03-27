package type_method_receiver

type Foo struct{}

func (f *Foo) Bar() { print("ok") }
func (f *Foo) Baz() { print("ok") }
func MockFunction() { print("ok") }
