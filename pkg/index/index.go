package index

import (
	"go/token"
	"strconv"
)

const (
	GoFileType = "go"
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
	case SymbolTypeDeclaration:
		return "declaration"
	case SymbolTypeDefinition:
		return "definition"
	default:
		return "unknown"
	}
}

const (
	SymbolTypeDeclaration SymbolType = iota + 1
	SymbolTypeDefinition
)

type SymbolId int
type DeclarationId int
type ModuleId int64
type FileId int64

func (id DeclarationId) String() string {
	return strconv.Itoa(int(id))
}

type Index interface {
	AddModule(moduleName string) (ModuleId, error)
	AddFile(moduleId *ModuleId, baseDir, path, filetype string, contents []byte) (FileId, error)

	AddSymbol(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType, start token.Position, end token.Position) (SymbolId, DeclarationId, error)
	FindSymbolId(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType) (SymbolId, DeclarationId, error)
	FindDeclarationId(name string, scope SymbolScope, symbolType SymbolType) ([]DeclarationId, error)
	GetAllSymbols() ([]SymbolDecl, error)

	AddReference(from FileId, to token.Position, toName string, start token.Position, end token.Position) error
	ResolveReferences() error
	GetAllReferencesNames() ([]*ReferenceNames, error)

	FindBuiltinDeclaration(name string) (FileId, token.Position, token.Position, error)
	FindFileId(path string) (FileId, error)

	Wait() error
	Close() error
}
