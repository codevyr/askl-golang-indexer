package index

import (
	"strings"

	"github.com/planetA/askl-golang-indexer/pkg/indexpb"
)

// Roles is the SET of things a referencing token does to its target, one bit
// each, zero meaning nothing is stated.
//
// EVERY ROLE IS A POSITIVE FACT ABOUT THE TEXT, AND ABSENCE STATES NOTHING.  A
// reference with no role is not "not a call": it is a reference the indexer
// did not examine for one, and reads exactly like a reference from an index
// written before roles existed.  So a role may be added to a kind of reference
// at any time, and none may be set on a guess.
//
// The bits are the wire format's (RefRole in index.proto).
type Roles uint32

const (
	// NoRole: nothing is stated about what the token does.
	NoRole Roles = 0
	// RoleCall: the token is the callee of a call -- `f(x)`, `r.Read(p)`.
	RoleCall = Roles(indexpb.RefRole_REF_ROLE_CALL)
	// RoleWrite: the target is assigned or initialised -- `total = n`.
	RoleWrite = Roles(indexpb.RefRole_REF_ROLE_WRITE)
	// RoleTake: its value or address is taken, not called -- `go run(cb)`.
	RoleTake = Roles(indexpb.RefRole_REF_ROLE_TAKE)
)

func (r Roles) String() string {
	if r == NoRole {
		return "-"
	}
	var said []string
	for _, role := range []struct {
		bit  Roles
		word string
	}{{RoleCall, "calls"}, {RoleWrite, "writes"}, {RoleTake, "takes"}} {
		if r&role.bit != 0 {
			said = append(said, role.word)
		}
	}
	return strings.Join(said, ",")
}

// Relation makes a row a fact between two symbols that no token of the source
// spells, placed on the declaration of its source.  Zero is an ordinary
// reference.
//
// A RELATION BELONGS TO ITS SOURCE ONLY.  The engine attributes a reference to
// every declaration around it, and a relation row to the one declaration it
// sits directly in.  So the row is written at a token INSIDE its source's own
// instance, with nothing narrower around it.
//
// The values are the wire format's (RefRelation in index.proto).
type Relation int32

const (
	// NoRelation: an ordinary reference.
	NoRelation = Relation(indexpb.RefRelation_REF_RELATION_NONE)
	// RelationHolds: the source is a dispatch point and the target something
	// stored in it -- `(io.Reader).Read -> (*File).Read`.
	RelationHolds = Relation(indexpb.RefRelation_REF_RELATION_HOLDS)
)

func (r Relation) String() string {
	switch r {
	case NoRelation:
		return "-"
	case RelationHolds:
		return "holds"
	}
	return indexpb.RefRelation(r).String()
}
