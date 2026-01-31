package generic_instantiation_app

import (
	generic_instantiation_lib "github.com/planetA/askl-golang-indexer/pkg/parser/test/src/generic_instantiation/lib"
)

type Doer interface {
	Foo()
}

func Call() {
	var d Doer
	var b generic_instantiation_lib.Box[int]
	d = b
	d.Foo()
}
