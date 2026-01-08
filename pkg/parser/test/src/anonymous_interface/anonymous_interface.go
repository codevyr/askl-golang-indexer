package anonymous_interface

// clearUnknown removes unknown fields from m where remover.Has reports true.
func ClearUnknown(remover interface {
	Has(int) bool
}) {
	num := 4
	if !remover.Has(num) {
		print("Hello, World!")
	}
}

type fieldNum int

func (n1 fieldNum) Has(n2 int) bool {
	return int(n1) == n2
}

func MockFunction() {
	var remover fieldNum = 4
	ClearUnknown(remover)
}
