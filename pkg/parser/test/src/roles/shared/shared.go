package shared

var Level int

func Notify(x int) int {
	return x
}

// Run is a variable that holds a function, as `ctrl.LoggerFrom` is.
var Run = Notify
