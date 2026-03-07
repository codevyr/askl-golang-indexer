package parser

import (
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestUniquePackages(t *testing.T) {
	pkgs := []*packages.Package{
		{ID: "a", PkgPath: "example.com/a"},
		{ID: "b", PkgPath: "example.com/b"},
		{ID: "a", PkgPath: "example.com/a"},
		{PkgPath: "example.com/c"},
		{PkgPath: "example.com/c"},
	}

	out := uniquePackages(pkgs)
	if len(out) != 3 {
		t.Fatalf("expected 3 unique packages, got %d", len(out))
	}
	if out[0].ID != "a" || out[1].ID != "b" || out[2].PkgPath != "example.com/c" {
		t.Fatalf("unexpected package order: %+v", out)
	}
}
