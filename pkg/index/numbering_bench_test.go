package index

// Why this exists: the numbering pass runs over every instance and every ref
// in a project, and the obvious implementation -- "for each instance, scan all
// the others for the narrowest container" -- is quadratic per object.  On a
// large project that is the difference between a pass lost in the noise and one
// that doubles indexing time, and nothing else in the suite would notice.
//
// Measured 2026-10-03 on this machine, sort-and-sweep:
//
//	1000 files, 153k instances, 200k refs ->  14 ms
//	5000 files, 765k instances,   1M refs ->  50 ms
//	5000 files,   3M instances,   4M refs -> 181 ms
//
// Against go/packages parsing a project that size, that is free.  A regression
// to per-instance scanning would show up here as orders of magnitude.

import (
	"fmt"
	"testing"

	"github.com/planetA/askl-golang-indexer/pkg/indexpb"
)

func benchProject(files, instPerFile, refsPerFile int) (*indexpb.Project, map[int64]int) {
	proj := &indexpb.Project{}
	rank := map[int64]int{1: 0, 2: 1}
	for f := 0; f < files; f++ {
		obj := &indexpb.Object{FilesystemPath: fmt.Sprintf("/p/d%d/f%d.go", f%50, f)}
		size := int32(instPerFile * 40)
		obj.SymbolInstances = append(obj.SymbolInstances,
			&indexpb.SymbolInstance{SymbolLocalId: 1, StartOffset: 0, EndOffset: size},
			&indexpb.SymbolInstance{SymbolLocalId: 2, StartOffset: 0, EndOffset: size},
			&indexpb.SymbolInstance{SymbolLocalId: int64(1000 + f), StartOffset: 0, EndOffset: size})
		rank[int64(1000+f)] = 2
		// Top-level decls, each with a nested closure and a local.
		for k := 0; k < instPerFile; k++ {
			s := int32(k * 40)
			obj.SymbolInstances = append(obj.SymbolInstances,
				&indexpb.SymbolInstance{SymbolLocalId: 5, StartOffset: s + 1, EndOffset: s + 38},
				&indexpb.SymbolInstance{SymbolLocalId: 6, StartOffset: s + 10, EndOffset: s + 30},
				&indexpb.SymbolInstance{SymbolLocalId: 7, StartOffset: s + 15, EndOffset: s + 20})
		}
		for k := 0; k < refsPerFile; k++ {
			off := int32((k * 40 / refsPerFile * instPerFile) % int(size))
			obj.Refs = append(obj.Refs, &indexpb.SymbolRef{
				ToSymbolLocalId: 5, FromOffsetStart: off, FromOffsetEnd: off + 3})
		}
		proj.Objects = append(proj.Objects, obj)
	}
	return proj, rank
}

func BenchmarkNumberObjectTrees(b *testing.B) {
	for _, c := range []struct{ files, inst, refs int }{
		{1000, 50, 200},
		{5000, 50, 200},
		{5000, 200, 800},
	} {
		b.Run(fmt.Sprintf("files=%d/decls=%d/refs=%d", c.files, c.inst, c.refs), func(b *testing.B) {
			proj, rank := benchProject(c.files, c.inst, c.refs)
			insts, refs := 0, 0
			for _, o := range proj.Objects {
				insts += len(o.SymbolInstances)
				refs += len(o.Refs)
			}
			b.ReportMetric(float64(insts), "instances")
			b.ReportMetric(float64(refs), "refs")
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				for _, obj := range proj.Objects {
					numberObjectTree(obj, rank, nil)
				}
			}
		})
	}
}
