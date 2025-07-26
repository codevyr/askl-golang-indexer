package return_values

type Integer interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

type Float interface {
	~float32 | ~float64
}

type Ordered interface {
	Integer | Float | ~string
}

func isNaN[T Ordered](x T) bool {
	return x != x
}

// cmpLess is a copy of cmp.Less from the Go 1.21 release.
func cmpLess[T Ordered](x, y T) bool {
	return (isNaN(x) && !isNaN(y)) || x < y
}

func insertionSortOrdered[E Ordered](data []E, a, b int) {
	for i := a + 1; i < b; i++ {
		for j := i; j > a && cmpLess(data[j], data[j-1]); j-- {
			data[j], data[j-1] = data[j-1], data[j]
		}
	}
}

func MockFunction() {
	dataUint := []uint{5, 3, 8, 6, 2}
	insertionSortOrdered(dataUint, 0, len(dataUint))
	dataFloat := []float64{5.1, 3.2, 8.3, 6.4, 2.5}
	insertionSortOrdered(dataFloat, 0, len(dataFloat))
	dataString := []string{"banana", "apple", "cherry", "date", "fig"}
	insertionSortOrdered(dataString, 0, len(dataString))
}
