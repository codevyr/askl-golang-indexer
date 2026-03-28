package var_pointer_ops

var GlobalInt int

func TakeAddr() *int {
	return &GlobalInt
}

func Deref(p *int) int {
	return *p
}
