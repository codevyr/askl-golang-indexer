package type_func_type

type Request struct{}
type Response struct{}
type Handler func(req *Request) (*Response, error)

func MockFunction() { print("ok") }
