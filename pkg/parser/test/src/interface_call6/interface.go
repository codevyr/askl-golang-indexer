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

	foo := func() (int, Mock) {
		return func() (int, Mock) {
			return func() (int, Mock) {
				return func() (int, Mock) {
					return 4, MockImpl{}
				}()
			}()
		}()
	}

	i, m = foo()
	m.MockFunction(i)
}
