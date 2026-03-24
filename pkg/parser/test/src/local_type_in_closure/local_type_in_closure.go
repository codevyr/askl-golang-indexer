package local_type_in_closure

func Outer() {
	inner := func(rt interface{}) {
		type canceler interface{ Cancel() }
		if v, ok := rt.(canceler); ok {
			v.Cancel()
		}
	}
	inner(nil)
}
