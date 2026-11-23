package kinds

// Sock satisfies Reader in a file of its own; nothing here hands one over.
type Sock struct{}

func (s Sock) Read() int {
	return 4
}
