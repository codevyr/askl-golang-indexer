package interface_call

type Mock1 interface {
}

type Mock2 interface {
	Mock1
	MockFunction(i int)
}

type MockImpl struct{}

var _ Mock2 = MockImpl{}

func (m MockImpl) MockFunction(i int) {
	print("Hello, World!", i)
}

func CallInterface() {
	var m Mock1
	var i int

	foo := func() (int, Mock1) {
		return 4, MockImpl{}
	}

	i, m = foo()
	print("Hello, World!", i, m)
}
