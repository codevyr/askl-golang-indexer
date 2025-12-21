package generic_functions

type PodSetReducer[R any] struct {
	fits R
}

func (psr *PodSetReducer[R]) Search() (R, bool) {
	return psr.fits, true
}

func NewPodSetReducer[R any](fits func() R) *PodSetReducer[R] {
	return &PodSetReducer[R]{
		fits: fits(),
	}
}

func Foo() {
	psr := NewPodSetReducer(func() int32 { return 4 })

	res, _ := psr.Search()
	print(res)
}
