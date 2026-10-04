package index

import (
	"go/token"
	"strconv"
)

const (
	GoFileType = "text/x-go"
)

type Config struct {
	project  string
	rootPath string
}

type Option interface {
	apply(config *Config)
}

func WithProject(project string) Option {
	return &projectOption{project: project}
}

type projectOption struct {
	project string
}

func (o *projectOption) apply(config *Config) {
	config.project = o.project
}

func WithRootPath(rootPath string) Option {
	return &rootPathOption{rootPath: rootPath}
}

type rootPathOption struct {
	rootPath string
}

func (o *rootPathOption) apply(config *Config) {
	config.rootPath = o.rootPath
}

type SymbolScope int

func (s SymbolScope) String() string {
	switch s {
	case ScopeGlobal:
		return "global"
	case ScopeLocal:
		return "local"
	default:
		return "unknown"
	}
}

const (
	ScopeLocal SymbolScope = iota + 1
	ScopeGlobal
)

type SymbolType int

func (s SymbolType) String() string {
	switch s {
	case SymbolTypeFunction:
		return "function"
	case SymbolTypeFile:
		return "file"
	case SymbolTypeModule:
		return "module"
	case SymbolTypeDirectory:
		return "directory"
	case SymbolTypeType:
		return "type"
	case SymbolTypeData:
		return "data"
	case SymbolTypeMacro:
		return "macro"
	case SymbolTypeField:
		return "field"
	default:
		return "unknown"
	}
}

const (
	SymbolTypeFunction  SymbolType = 1
	SymbolTypeFile      SymbolType = 2
	SymbolTypeModule    SymbolType = 3
	SymbolTypeDirectory SymbolType = 4
	SymbolTypeType      SymbolType = 5
	SymbolTypeData      SymbolType = 6
	SymbolTypeMacro     SymbolType = 7 // not emitted by Go indexer; defined for completeness
	SymbolTypeField     SymbolType = 8
)

type InstanceType int

const (
	InstanceTypeDefinition   InstanceType = 1
	InstanceTypeDeclaration  InstanceType = 2
	InstanceTypeExpansion    InstanceType = 3
	InstanceTypeSentinel     InstanceType = 4
	InstanceTypeContainment  InstanceType = 5
	InstanceTypeSource       InstanceType = 6
	InstanceTypeHeader       InstanceType = 7
	InstanceTypeBuild        InstanceType = 8
	InstanceTypeFile         InstanceType = 9
	InstanceTypeDocumentation InstanceType = 10
)

// Span is a half-open [Start, End) byte range inside an instance's extent.
type Span struct {
	Start int
	End   int
}

// Spans are the finer ranges inside an instance's extent, each OPTIONAL: a nil
// field means "this producer does not know", which is how the wire format reads
// an absent span too.  The extent itself stays the authoritative thing an
// instance occupies; these only say which part of it is the identifier, which
// the declaration, which the body.
//
// They travel with AddSymbol rather than arriving in a follow-up call so an
// instance cannot be created with its spans forgotten.
type Spans struct {
	// Name is the identifier alone -- `Run` in `func Run(x int) int { ... }`.
	Name *Span
	// Signature is the declaration without the body, so a multi-line one can
	// be shown whole.
	Signature *Span
	// Content is the body alone.
	Content *Span
}

// span is the constructor used by producers: both ends are byte offsets from
// the start of the object.
func NewSpan(start, end int) *Span {
	return &Span{Start: start, End: end}
}

type SymbolId int
type SymbolInstanceId int

// NoParent means "derive my parent from the ranges", which is the right answer
// almost always: a declaration sits inside the text of whatever declares it.
//
// Pass a real instance instead when the ranges CANNOT say it. In
// `var First, Second = f1, f2` the names share one range and First's identifier
// is not contiguous with its own initialiser, so no extent makes f1 First's and
// f2 Second's -- the parser knows which is which and nothing else can.
const NoParent SymbolInstanceId = 0
type ModuleId int64
type FileId int64

func (id SymbolInstanceId) String() string {
	return strconv.Itoa(int(id))
}

type Index interface {
	AddModule(moduleName string) (ModuleId, error)
	AddModuleImport(fromModuleId ModuleId, toModuleName string, fromFileId FileId, startOffset, endOffset int) error
	AddFile(moduleId *ModuleId, baseDir, path, filetype string, contents []byte) (FileId, error)

	// AddSymbol records one instance of a symbol. `parent` states its place in
	// the containment tree when the ranges cannot; see NoParent.
	AddSymbol(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType, instanceType InstanceType, start token.Position, end token.Position, spans Spans, parent SymbolInstanceId) (SymbolId, SymbolInstanceId, error)
	FindSymbolId(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType) (SymbolId, SymbolInstanceId, error)
	FindSymbolInstanceId(name string, scope SymbolScope, symbolType SymbolType) ([]SymbolInstanceId, error)
	GetAllSymbols() ([]SymbolDecl, error)

	// AddReference records a use of a symbol.  start/end delimit the REFERENCING
	// TOKEN -- `a.Target` in `a.Target(x)` -- and full, when given, the whole
	// construct around it.  A reference is a point in the containment order, not
	// an interval, so these ranges are what order and display it; they never
	// decide what contains it.
	AddReference(from FileId, to token.Position, toName string, start token.Position, end token.Position, full *Span) error
	ResolveReferences() error
	GetAllReferencesNames() ([]*ReferenceNames, error)

	FindBuiltinInstance(name string) (FileId, token.Position, token.Position, error)
	FindFileId(path string) (FileId, error)

	Wait() error
	Close() error
}
