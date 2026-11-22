package roles

import "github.com/planetA/askl-golang-indexer/pkg/parser/test/src/roles/shared"

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

// --- A function used as a value ---

type Handler func(int) int

type Table struct {
	Run Handler
}

func double(x int) int {
	return x * 2
}

func register(h Handler) {}

func Passed() {
	register(double)
}

func Assigned() Handler {
	h := double
	return h
}

func Returned() Handler {
	return double
}

func Stored() (Table, map[string]Handler) {
	return Table{Run: double}, map[string]Handler{"a": double}
}

func MethodValue(f *File) func() int {
	return f.Read
}

func MethodExpr() func(*File) int {
	return (*File).Read
}

func ThroughValue(r Reader) func() int {
	return r.Read
}

func Spawned() {
	go register(double)
	defer double(1)
}

// A generic function instantiated and called is called.
func pick[T any](x T) T {
	return x
}

func Instantiated() (int, func(int) int) {
	return pick[int](1), pick[int]
}

// Parentheses change nothing about what is called.
func Parenthesised() int {
	return (pick[int])(1)
}

// --- Package-level data: written, read, called ---

var Count int

var Hook Handler = double

var Config struct {
	Debug bool
	Sizes [2]int
	Names map[string]int
}

const Max = 10

func ReadsCount() int {
	return Count
}

func WritesCount(n int) {
	Count = n
}

func BumpsCount() {
	Count++
}

func AddsCount(n int) {
	Count += n
}

func PointsAtCount() *int {
	return &Count
}

func CallsHook() int {
	return Hook(1)
}

func SetsHook() {
	Hook = double
}

func ReadsMax() int {
	return Max
}

// A field or an array element of a variable is part of the variable: storing
// into it writes the variable.  A map element is somewhere else, and finding
// it reads the variable.
func WritesInside() {
	Config.Debug = true
	Config.Sizes[0] = 1
	Config.Names["a"] = 1
}

func RangesInto(xs []int) {
	for Count = range xs {
	}
	for _, Count = range xs {
	}
}

// A local variable that names a function literal, assigned again: the token
// is the variable written, and says nothing of the function.
func Reassigned() int {
	inner := func(x int) int {
		return x
	}
	inner = double
	return inner(1)
}

// --- What has no callee token, and what leads elsewhere ---

var Hooks []Handler

type Node struct {
	Next *Node
}

var Head *Node

// An element of `Hooks` is called: no token is the callee, and `Hooks` is
// read.  `(double)(1)` calls `double`.
func CallsElement() int {
	return Hooks[0](1) + (double)(1)
}

// `Head.Next` is somewhere `Head` points to: storing there reads `Head`.
func WritesThrough() {
	Head.Next = nil
}

// --- Another package's names ---

func Qualified() int {
	shared.Level = 2
	shared.Level++
	register(shared.Notify)
	return shared.Run(1) + shared.Notify(1)
}

// --- What has no reference, or one that is not a use ---

// The method of an interface type written here is declared, not used.
func LocalInterface(x any) bool {
	_, ok := x.(interface{ Read() int })
	return ok
}

// `error.Error` is a function of the universe and has no declaration.
func ErrText(err error) func() string {
	return err.Error
}

type Box[T any] struct {
	v T
}

func (b Box[T]) Get() T {
	return b.v
}

// A method value of an instantiated type takes the generic method.
func BoxValue(b Box[int]) func() int {
	return b.Get
}
