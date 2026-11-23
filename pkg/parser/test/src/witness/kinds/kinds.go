package kinds

// Reader is what is expected; File, Pipe and Sock are what is handed over.
type Reader interface {
	Read() int
}

type ReadCloser interface {
	Read() int
	Close()
}

type File struct{}

func (f *File) Read() int {
	return 1
}

type Pipe struct{}

func (p Pipe) Read() int {
	return 2
}

type Stream struct{}

func (s *Stream) Read() int {
	return 3
}

func (s *Stream) Close() {}

// A package-level declaration that says it and keeps nothing.
var _ Reader = Pipe{}

// A returned value.
func Open() Reader {
	return &File{}
}
