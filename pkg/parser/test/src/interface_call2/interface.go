package interface_call

type Mock interface {
	MockFunction()
}

type MockImpl struct{}

func (m MockImpl) MockFunction() {
	print("Hello, World!")
}

func CallInterface() {
	var m Mock
	m = MockImpl{}
	m.MockFunction()
}
