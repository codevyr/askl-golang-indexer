package primitive_types

func MockFunction() {
	i := int32(4)
	_ = i
	c := make([]int, 0)
	c = append(c, 1, 2, 3)
	for _, v := range c {
		print(v)
	}
}
