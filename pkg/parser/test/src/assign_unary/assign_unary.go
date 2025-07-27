package assign_unary

type Mock interface {
	MockFunction()
}

type MockImpl struct {
	ShuffleAddressListForTesting any
}

func (m MockImpl) MockFunction() {
	checkedMetricChan := make(chan Mock, 1)
	cmc := checkedMetricChan
	for {
		select {
		case metric, ok := <-cmc:
			if !ok {
				return
			}
			metric.MockFunction()
		default:
			return
		}
	}
}

func CallInterface() {
	m := MockImpl{}
	m.MockFunction()
	if m.ShuffleAddressListForTesting == nil {
		panic("ShuffleAddressListForTesting should not be nil")
	}
}
