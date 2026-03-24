package nested_func

func OuterFunc() {
	inner := func(x int) int {
		print(x)
		return x + 1
	}
	_ = inner(1)

	go func() {
		print("goroutine")
	}()
}
