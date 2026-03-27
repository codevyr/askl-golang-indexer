package type_struct_embed_interface

type Writer interface{ Write() }
type LogWriter struct {
	Writer
	prefix string
}

func MockFunction() { print("ok") }
