package index

import (
	"cmp"
	"fmt"
	"go/token"
	"strings"

	"github.com/onsi/gomega/types"
)

type SymbolDecl struct {
	ModuleId   ModuleId
	FileId     FileId
	Name       string
	Scope      SymbolScope
	SymbolType SymbolType
	Start      token.Position
	End        token.Position
}

func NewSymbol(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType, start *token.Position, end *token.Position) *SymbolDecl {
	s := &SymbolDecl{
		ModuleId:   moduleId,
		FileId:     fileId,
		Name:       name,
		Scope:      scope,
		SymbolType: symbolType,
	}
	if start != nil {
		s.Start = *start
	}
	if end != nil {
		s.End = *end
	}
	return s
}

func (s *SymbolDecl) Compare(other *SymbolDecl) int {
	if s.ModuleId != other.ModuleId {
		return cmp.Compare(int64(s.ModuleId), int64(other.ModuleId))
	}
	if s.FileId != other.FileId {
		return cmp.Compare(int64(s.FileId), int64(other.FileId))
	}
	if s.Name != other.Name {
		return strings.Compare(s.Name, other.Name)
	}
	if s.Scope != other.Scope {
		return cmp.Compare(s.Scope, other.Scope)
	}
	return 0
}

type SymbolMatcher struct {
	Expected *SymbolDecl
}

func RepresentSymbol(expected *SymbolDecl) types.GomegaMatcher {
	return &SymbolMatcher{
		Expected: expected,
	}
}

func (matcher *SymbolMatcher) Match(actual any) (success bool, err error) {
	s, ok := actual.(SymbolDecl)
	if !ok {
		return false, fmt.Errorf("SymbolMatcher matcher expects a Symbol, got %T", actual)
	}

	rest := strings.HasSuffix(s.Name, matcher.Expected.Name) &&
		s.Scope == matcher.Expected.Scope
	if !rest {
		return false, nil
	}

	zeroPosition := token.Position{}
	if matcher.Expected.Start == zeroPosition && matcher.Expected.End == zeroPosition {
		return true, nil
	}

	return s.Start == matcher.Expected.Start &&
		s.End == matcher.Expected.End, nil
}

func (matcher *SymbolMatcher) FailureMessage(actual any) (message string) {
	var actualString string
	if s, ok := actual.(SymbolDecl); ok {
		actualString = fmt.Sprintf("{\n\tmoduleId: %d,\n\tfileId: %d,\n\tname: %s,\n\tscope: %s,\n\tstart: %v,\n\tend: %v\n}",
			s.ModuleId, s.FileId, s.Name, s.Scope, s.Start, s.End)
	} else {
		actualString = fmt.Sprintf("%#v", actual)
	}

	var expectedString string
	if matcher.Expected != nil {
		expectedString = fmt.Sprintf("{\n\tmoduleId: %d,\n\tfileId: %d,\n\tname: %s,\n\tscope: %s,\n\tstart: %v,\n\tend: %v\n}",
			matcher.Expected.ModuleId, matcher.Expected.FileId, matcher.Expected.Name,
			matcher.Expected.Scope, matcher.Expected.Start, matcher.Expected.End)
	} else {
		expectedString = "nil"
	}
	return fmt.Sprintf("Expected\n\t%s\nto contain the Symbol representation of\n\t%s", actualString, expectedString)
}

func (matcher *SymbolMatcher) NegatedFailureMessage(actual any) (message string) {
	var actualString string
	if s, ok := actual.(SymbolDecl); ok {
		actualString = fmt.Sprintf("{\n\tmoduleId: %d,\n\tfileId: %d,\n\tname: %s,\n\tscope: %s,\n\tstart: %v,\n\tend: %v\n}",
			s.ModuleId, s.FileId, s.Name, s.Scope, s.Start, s.End)
	} else {
		actualString = fmt.Sprintf("%#v", actual)
	}

	var expectedString string
	if matcher.Expected != nil {
		expectedString = fmt.Sprintf("{\n\tmoduleId: %d,\n\tfileId: %d,\n\tname: %s,\n\tscope: %s,\n\tstart: %v,\n\tend: %v\n}",
			matcher.Expected.ModuleId, matcher.Expected.FileId, matcher.Expected.Name,
			matcher.Expected.Scope, matcher.Expected.Start, matcher.Expected.End)
	} else {
		expectedString = "nil"
	}
	return fmt.Sprintf("Expected\n\t%s\nnot to contain the Symbol representation of\n\t%s", actualString, expectedString)
}

type ReferenceNames struct {
	From string
	To   string
}

func NewReferenceNames(from, to string) *ReferenceNames {
	return &ReferenceNames{
		From: from,
		To:   to,
	}
}

type ReferenceMatcher struct {
	Expected *ReferenceNames
}

func RepresentReference(expected *ReferenceNames) types.GomegaMatcher {
	return &ReferenceMatcher{
		Expected: expected,
	}
}

func (matcher *ReferenceMatcher) Match(actual any) (success bool, err error) {
	r, ok := actual.(*ReferenceNames)
	if !ok {
		return false, fmt.Errorf("ReferenceMatcher matcher expects a ReferenceNames, got %T", actual)
	}

	if !strings.HasSuffix(r.From, matcher.Expected.From) || !strings.HasSuffix(r.To, matcher.Expected.To) {
		return false, nil
	}
	return true, nil
}

func (matcher *ReferenceMatcher) FailureMessage(actual any) (message string) {
	var actualString string
	if r, ok := actual.(ReferenceNames); ok {
		actualString = fmt.Sprintf("{\n\tFrom: %s,\n\tTo: %s\n}", r.From, r.To)
	} else {
		actualString = fmt.Sprintf("%#v", actual)
	}
	var expectedString string
	if matcher.Expected != nil {
		expectedString = fmt.Sprintf("{\n\tFrom: %s,\n\tTo: %s\n}", matcher.Expected.From, matcher.Expected.To)
	} else {
		expectedString = "nil"
	}
	return fmt.Sprintf("Expected\n\t%s\nto contain the ReferenceNames representation of\n\t%s", actualString, expectedString)
}

func (matcher *ReferenceMatcher) NegatedFailureMessage(actual any) (message string) {
	var actualString string
	if r, ok := actual.(ReferenceNames); ok {
		actualString = fmt.Sprintf("{\n\tFrom: %s,\n\tTo: %s\n}", r.From, r.To)
	} else {
		actualString = fmt.Sprintf("%#v", actual)
	}
	var expectedString string
	if matcher.Expected != nil {
		expectedString = fmt.Sprintf("{\n\tFrom: %s,\n\tTo: %s\n}", matcher.Expected.From, matcher.Expected.To)
	} else {
		expectedString = "nil"
	}
	return fmt.Sprintf("Expected\n\t%s\nnot to contain the ReferenceNames representation of\n\t%s", actualString, expectedString)
}
