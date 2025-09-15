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

func (m *MockImpl) CallInterface() {
	var mi Mock2
	var i int

	foo := func(m Mock2) (int, Mock2) {
		return 4, m
	}

	i, mi = foo(m)
	print("Hello, World!", i, mi)
}
