package interface_call

type Mock interface {
	MockFunction()
}

type MockImpl struct {
	ShuffleAddressListForTesting any
}

func (m MockImpl) MockFunction() {
	m.ShuffleAddressListForTesting = func(n int, swap func(i, j int)) {}
}

func CallInterface() {
	m := MockImpl{}
	m.MockFunction()
	if m.ShuffleAddressListForTesting == nil {
		panic("ShuffleAddressListForTesting should not be nil")
	}
}
