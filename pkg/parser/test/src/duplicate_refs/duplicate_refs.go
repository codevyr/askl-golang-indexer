package duplicate_refs

type PathError struct{}

func (p *PathError) Error() string {
	return "boom"
}

func DuplicateRefs() {
	var err error
	err = &PathError{}
	err = &PathError{}
	_ = err
}
