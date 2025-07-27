package return_nested

type sysDialer struct {
	Control func(network, address string) error
}

func (sd *sysDialer) dialUnix() (*int, error) {
	ctrlCtxFn := func(network, address string) error {
		return sd.Control(network, address)
	}

	return nil, ctrlCtxFn("unix", "address")
}

func MockFunction() {
	var dialer sysDialer
	_, err := dialer.dialUnix()
	if err != nil {
		print("Hello, World!")
	}
}
