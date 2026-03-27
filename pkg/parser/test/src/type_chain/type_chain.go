package type_chain

type Real struct{}
type Alias = Real
type Consumer struct{ a Alias }

func MockFunction() { print("ok") }
