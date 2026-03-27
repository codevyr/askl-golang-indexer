package parser

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/planetA/askl-golang-indexer/pkg/logging"
	"golang.org/x/tools/go/packages"
)

// typeRef holds a reference to a named type and the use-site positions.
type typeRef struct {
	typeName *types.TypeName
	usePos   token.Pos
	useEnd   token.Pos
}

// extractNamedTypes recursively walks one or more ast.Expr and collects
// *types.TypeName references via pkg.TypesInfo.Uses. Multiple expressions
// share a single dedup map, preventing duplicate refs across e.g. generic
// type parameter constraints. Returns both the type definition and the
// use-site positions needed for AddReference.
func extractNamedTypes(pkg *packages.Package, exprs ...ast.Expr) []typeRef {
	seen := make(map[types.Object]struct{})
	var refs []typeRef

	// tryAdd checks whether an identifier resolves to a collectible *types.TypeName
	// and appends it to refs if so. Returns true if the ref was added.
	tryAdd := func(ident *ast.Ident) {
		obj, ok := pkg.TypesInfo.Uses[ident]
		if !ok {
			return
		}
		tn, ok := obj.(*types.TypeName)
		if !ok {
			return
		}
		// Skip type parameters (T in generics)
		if _, isTypeParam := tn.Type().(*types.TypeParam); isTypeParam {
			return
		}
		if _, dup := seen[obj]; dup {
			return
		}
		seen[obj] = struct{}{}
		// Skip builtin primitive types (int, string, etc.) by default.
		// Callers handle builtin type refs separately when needed.
		if tn.Pkg() == nil {
			return
		}
		// TODO: local types (defined inside functions) are not yet supported
		// as reference targets — their full names include the parent function
		// and can't be reliably reconstructed from *types.TypeName alone.
		if tn.Parent() != tn.Pkg().Scope() {
			logging.Warnf("Skipping type reference to local type %s.%s: local type references not yet implemented", tn.Pkg().Path(), tn.Name())
			return
		}
		refs = append(refs, typeRef{typeName: tn, usePos: ident.Pos(), useEnd: ident.End()})
	}

	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		if e == nil {
			return
		}
		switch n := e.(type) {
		case *ast.Ident:
			tryAdd(n)
		case *ast.SelectorExpr:
			tryAdd(n.Sel)

		case *ast.StarExpr:
			walk(n.X)
		case *ast.ArrayType:
			walk(n.Elt)
		case *ast.MapType:
			walk(n.Key)
			walk(n.Value)
		case *ast.ChanType:
			walk(n.Value)
		case *ast.FuncType:
			if n.Params != nil {
				for _, field := range n.Params.List {
					walk(field.Type)
				}
			}
			if n.Results != nil {
				for _, field := range n.Results.List {
					walk(field.Type)
				}
			}
		case *ast.StructType:
			if n.Fields != nil {
				for _, field := range n.Fields.List {
					walk(field.Type)
				}
			}
		case *ast.InterfaceType:
			// Only recurse on embedded types (no method names)
			if n.Methods != nil {
				for _, field := range n.Methods.List {
					if len(field.Names) == 0 {
						walk(field.Type)
					}
					// Do NOT recurse into method signatures — those belong to
					// the method's FUNCTION symbol.
				}
			}
		case *ast.IndexExpr:
			walk(n.X)
			walk(n.Index)
		case *ast.IndexListExpr:
			walk(n.X)
			for _, idx := range n.Indices {
				walk(idx)
			}
		case *ast.ParenExpr:
			walk(n.X)
		case *ast.Ellipsis:
			walk(n.Elt)
		case *ast.UnaryExpr:
			// Handles ~T in type constraints
			walk(n.X)
		case *ast.BinaryExpr:
			// Handles A | B union types in constraints
			walk(n.X)
			walk(n.Y)
		}
	}

	for _, expr := range exprs {
		walk(expr)
	}
	return refs
}
