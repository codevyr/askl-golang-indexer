package return_pointer

type transport interface {
	SupportsUnixFDs() bool
}

func newNonceTcpTransport(keys string) (transport, error) {
	return NewConn()
}

type Conn struct{}

func (c *Conn) SupportsUnixFDs() bool {
	return false
}

func NewConn() (*Conn, error) {
	return nil, nil
}

func MockFunction() {
	_, err := newNonceTcpTransport("keys")
	if err != nil {
		print("Hello, World!")
	}
}
