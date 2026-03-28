package const_usage

const MaxRetries int = 3

func ReadMax() int {
	return MaxRetries
}

func UseMax(n int) bool {
	return n < MaxRetries
}

func DoubleCheck() bool {
	return MaxRetries > 0 && MaxRetries < 100
}
