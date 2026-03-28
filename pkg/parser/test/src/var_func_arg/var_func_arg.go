package var_func_arg

var GlobalVal int

func consume(x int) {}

func Use() {
	consume(GlobalVal)
}
