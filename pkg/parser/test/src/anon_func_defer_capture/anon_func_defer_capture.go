package anon_func_defer_capture

func Outer() {
	cleanup := func() {
		print("cleanup")
	}

	defer func() {
		cleanup()
	}()

	print("work")
}
