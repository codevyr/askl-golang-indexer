package type_self_ref

type Node struct {
	Next  *Node
	Value int
}

func MockFunction() { print("ok") }
