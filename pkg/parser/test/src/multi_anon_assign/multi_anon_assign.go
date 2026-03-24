package multi_anon_assign

func Outer() {
	double, triple := func(x int) int {
		return x * 2
	}, func(x int) int {
		return x * 3
	}
	_ = double(1)
	_ = triple(1)
}
