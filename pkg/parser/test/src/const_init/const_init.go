package const_init

const DefaultName string = "init"

var Name string

func init() {
	Name = DefaultName
}
