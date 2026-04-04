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
)

type SymbolId int
type SymbolInstanceId int
type ModuleId int64
type FileId int64

func (id SymbolInstanceId) String() string {
	return strconv.Itoa(int(id))
}

type Index interface {
	AddModule(moduleName string) (ModuleId, error)
	AddModuleImport(fromModuleId ModuleId, toModuleName string, fromFileId FileId, startOffset, endOffset int) error
	AddFile(moduleId *ModuleId, baseDir, path, filetype string, contents []byte) (FileId, error)

	AddSymbol(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType, instanceType InstanceType, start token.Position, end token.Position) (SymbolId, SymbolInstanceId, error)
	FindSymbolId(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType) (SymbolId, SymbolInstanceId, error)
	FindSymbolInstanceId(name string, scope SymbolScope, symbolType SymbolType) ([]SymbolInstanceId, error)
	GetAllSymbols() ([]SymbolDecl, error)

	AddReference(from FileId, to token.Position, toName string, start token.Position, end token.Position) error
	ResolveReferences() error
	GetAllReferencesNames() ([]*ReferenceNames, error)

	FindBuiltinInstance(name string) (FileId, token.Position, token.Position, error)
	FindFileId(path string) (FileId, error)

	Wait() error
	Close() error
}
