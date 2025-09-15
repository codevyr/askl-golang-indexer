package call_expr_parser

type Writer interface {
	Write(p []byte) (n int, err error)
}

func Fprintln(w Writer, a ...any) (n int, err error) {
	return w.Write([]byte("Hello, World!"))
}

var Stderr = &File{}

type File struct{}

func (f *File) Write(b []byte) (n int, err error) {
	return 0, nil
}

func CallInterface() {
	url := "http://example.com"
	print("Hello, World!", url)
	Fprintln(Stderr, "httptest: serving on", url)
}
