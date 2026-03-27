package type_collections

type Entry struct{}
type Cache map[string]Entry
type Entries []Entry
type EventChan chan Entry
type EntryPtr *Entry

func MockFunction() { print("ok") }
