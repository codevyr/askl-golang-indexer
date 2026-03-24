package anon_func_reassign

func apply(fn func(int) int, x int) int {
	return fn(x)
}

func Outer() {
	transform := func(x int) int {
		return x * 2
	}
	_ = apply(transform, 5)

	transform = func(x int) int {
		return x * 3
	}
	_ = apply(transform, 5)
}
