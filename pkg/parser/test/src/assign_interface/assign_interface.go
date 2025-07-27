package assign_interface

type Types struct{}

var GlobalTypes *Types = new(Types)

func (t *Types) FindExtensionByName() (int, int) {
	return 0, 0
}

type UnmarshalInput = struct {
	Resolver interface {
		FindExtensionByName() (int, int)
	}
	Depth int
}

type UnmarshalInput2 = struct {
	Resolver interface {
		FindExtensionByName() (int, int)
	}
	Depth int
}

func MockFunction() {
	var in UnmarshalInput
	if in.Resolver == nil {
		in.Resolver = GlobalTypes
	}
}
