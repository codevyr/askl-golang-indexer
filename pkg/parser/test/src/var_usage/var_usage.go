package var_usage

var Counter int

func ReadCounter() int {
	return Counter
}

func WriteCounter(n int) {
	Counter = n
}

func IncrCounter() {
	Counter = Counter + 1
}
