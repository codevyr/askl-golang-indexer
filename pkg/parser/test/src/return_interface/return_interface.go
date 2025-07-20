package return_values

type exporter func(v any, i int) any

type MessageInfo struct {
	Exporter exporter
}

type AdditionalPropertiesItem struct {
	a struct{}
	b int32
	c string
	d []byte
	e map[int]int
	f map[string]string
	g chan int
}

func Foo() MessageInfo {
	var i MessageInfo
	i.Exporter = func(v interface{}, i int) interface{} {
		switch v := v.(*AdditionalPropertiesItem); i {
		case 0:
			return &v.a
		case 1:
			return &v.b
		case 2:
			return &v.c
		case 3:
			return &v.d
		case 4:
			return &v.e
		case 5:
			return &v.f
		case 6:
			return &v.g
		default:
			return nil
		}
	}
	return i
}

func MockFunction() {
	i := Foo()
	if i.Exporter != nil {
		print("Hello, World!")
	}
}
