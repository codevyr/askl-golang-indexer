package anon_func_returned

func Maker() func(int) int {
	return func(x int) int {
		return x * 2
	}
}

func Caller() {
	fn := Maker()
	_ = fn(5)
}
