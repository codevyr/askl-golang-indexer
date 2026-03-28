package const_func_arg

const GlobalVal int = 10

func consume(x int) {}

func Use() {
	consume(GlobalVal)
}
