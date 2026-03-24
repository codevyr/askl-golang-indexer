package anon_func_callback

func apply(fn func(int) int, x int) int {
	return fn(x)
}

func Outer() {
	double := func(x int) int {
		return x * 2
	}
	_ = apply(double, 5)
}
