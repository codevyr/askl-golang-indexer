package consumer

import (
	"unsafe"

	"github.com/planetA/askl-golang-indexer/pkg/parser/test/src/module_imports/provider"
)

func UseProvider() string {
	return provider.Hello()
}

func UseBuiltinAndUnsafe() uintptr {
	// Use builtin functions (len, make)
	s := make([]int, 10)
	_ = len(s)

	// Use unsafe
	var x int
	return uintptr(unsafe.Pointer(&x))
}
