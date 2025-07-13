package interface_call

type Mock interface {
	MockFunction(i int)
}

type MockImpl struct{}

func (m MockImpl) MockFunction(i int) {
	print("Hello, World!", i)
}

func foo() (int, Mock) {
	return 4, MockImpl{}
}

func CallInterface() {
	var m Mock
	var i int
	i, m = foo()
	m.MockFunction(i)
}
