package inline_type_assert

func Outer(rt interface{}) {
	if m, ok := rt.(interface{ GetName() string }); ok {
		print(m.GetName())
	}
}
