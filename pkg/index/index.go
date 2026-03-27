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
	default:
		return "unknown"
	}
}

const (
	SymbolTypeFunction SymbolType = iota + 1
	SymbolTypeFile
	SymbolTypeModule
	SymbolTypeDirectory
	SymbolTypeType
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

	AddSymbol(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType, start token.Position, end token.Position) (SymbolId, SymbolInstanceId, error)
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
