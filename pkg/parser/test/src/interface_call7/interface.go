package interface_call

type Mock1 interface {
}

type Mock2 interface {
	Mock1
	MockFunction(i int)
}

type MockImpl struct{}

func (m MockImpl) MockFunction(i int) {
	print("Hello, World!", i)
}

func CallInterface() {
	var m Mock2
	var i int

	foo := func(m Mock2) (int, Mock2) {
		return 4, m
	}

	i, m = foo(MockImpl{})
	print("Hello, World!", i, m)
}
