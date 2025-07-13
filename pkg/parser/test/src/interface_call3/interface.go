package interface_call

type Mock interface {
	MockFunction(i int)
}

type MockImpl struct{}

func (m MockImpl) MockFunction(i int) {
	print("Hello, World!", i)
}

func CallInterface() {
	var m Mock
	var i int
	i, m = 4, MockImpl{}
	m.MockFunction(i)
}
