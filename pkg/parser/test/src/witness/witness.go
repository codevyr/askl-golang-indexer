package witness

import (
	"fmt"
	"io"
	"os"

	"github.com/planetA/askl-golang-indexer/pkg/parser/test/src/witness/kinds"
)

func use(r kinds.Reader) int {
	return r.Read()
}

func open() (*kinds.File, error) {
	return &kinds.File{}, nil
}

// The right side of an assignment, twice, and an argument.
func Assigned() int {
	var r kinds.Reader
	r = &kinds.File{}
	r = &kinds.File{}
	return use(r) + use(&kinds.File{})
}

// One call yields the value and an error: the call is what is handed over.
func Yielded() (kinds.Reader, error) {
	var r kinds.Reader
	var err error
	r, err = open()
	return r, err
}

func both() (int, *kinds.File) {
	return 0, &kinds.File{}
}

// The same when the interface is not the first thing the call yields.
func YieldedSecond() (int, kinds.Reader) {
	var n int
	var r kinds.Reader
	n, r = both()
	return n, r
}

// A returned value of a type declared in another file of another package.
func Returned(pipe bool) kinds.Reader {
	if pipe {
		return kinds.Pipe{}
	}
	return kinds.Sock{}
}

// A named interface handed over where a smaller one is expected relates
// nothing new: what it holds is already said of the wider one.
func Narrowed() kinds.Reader {
	var rc kinds.ReadCloser = &kinds.Stream{}
	return rc
}

// An interface type written out and handed over where a named one is
// expected: the named method holds the written one.
func Unnamed(x interface{ Read() int }) kinds.Reader {
	return x
}

// A type parameter is handed over as its constraint.
func Constrained[T interface{ Read() int }](x T) kinds.Reader {
	return x
}

// --- Across the edge of the module ---

type Name string

func (n Name) String() string {
	return string(n)
}

type Failure struct{}

func (f Failure) Error() string {
	return "failed"
}

// A type of this module handed over where an interface of the standard
// library, and of the universe, is expected.
func Outward() (fmt.Stringer, error) {
	return Name("a"), Failure{}
}

// A type of the standard library handed over where one of its own interfaces
// is expected, in this module's code.
func Inward() io.Writer {
	return os.Stdout
}
