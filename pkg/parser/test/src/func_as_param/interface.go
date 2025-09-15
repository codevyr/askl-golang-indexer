package interface_call

type ResponseWriter interface{}
type Request struct{}

type HandlerFunc func(ResponseWriter, *Request) (int, int)

type FooType struct{}

func (f FooType) Foo(w ResponseWriter, r *Request) (int, int) {
	return 2, 2
}

func CallInterface() {
	var mi int
	var i int

	h := HandlerFunc(func(w ResponseWriter, r *Request) (int, int) {
		return 4, 5
	})
	mi, i = h(nil, nil)
	foo := FooType{}
	ni := HandlerFunc(foo.Foo)
	print("Hello, World!", i, mi, ni)
}
