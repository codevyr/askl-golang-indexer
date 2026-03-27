package type_interface_embed

type Reader interface{ Read() }
type Writer interface{ Write() }
type ReadWriter interface {
	Reader
	Writer
}

func MockFunction() { print("ok") }
