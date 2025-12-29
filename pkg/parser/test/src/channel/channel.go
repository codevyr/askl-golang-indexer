package channel

func Foo() {
	msg := make(chan interface{})

	m, ok := (<-msg)
	println(m, ok)
	m, ok = ((((<-msg))))
	println(m, ok)
}
