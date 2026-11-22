package roles

type Reader interface {
	Read() int
}

type File struct{}

func (f *File) Read() int {
	return helper()
}

type Pipe struct{}

func (p Pipe) Read() int {
	return 0
}

func helper() int {
	return 1
}

var Limit = 3

func Direct() int {
	return helper() + Limit
}

func Method(f *File) int {
	return f.Read()
}

func Through(r Reader) int {
	return r.Read()
}

func Bind() Reader {
	var r Reader
	r = &File{}
	r = Pipe{}
	r = &File{}
	return r
}

func apply(fn func(int) int) int {
	return fn(1)
}

func Closure() int {
	twice := func(x int) int {
		return x * 2
	}
	return twice(2) + apply(twice)
}
