package app

import "github.com/planetA/askl-golang-indexer/pkg/parser/test/src/type_cross_pkg/types"

type Extended struct {
	B *types.Base
}

func MockFunction() { print("ok") }
