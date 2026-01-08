package index

import (
	"fmt"
	"go/token"
	"strconv"
)

const (
	goFileType = "go"
)

type Config struct {
	project         string
	indexPath       string
	extraSqliteArgs string
	recreate        bool
}

type Option interface {
	apply(config *Config)
}

type indexPathOption struct {
	indexPath       string
	extraSqliteArgs string
	project         string
}

func WithIndexPath(indexPath string) Option {
	return &indexPathOption{
		indexPath: indexPath,
	}
}

func WithProject(project string) Option {
	return &indexPathOption{
		project: project,
	}
}

type JournalMode string

const (
	JournalModeDelete   JournalMode = "DELETE"
	JournalModeTruncate JournalMode = "TRUNCATE"
	JournalModePersist  JournalMode = "PERSIST"
	JournalModeMemory   JournalMode = "MEMORY"
	JournalModeWal      JournalMode = "WAL"
	JournalModeOff      JournalMode = "OFF"
)

func WithJournal(mode JournalMode) Option {
	return &indexPathOption{
		extraSqliteArgs: fmt.Sprintf("_journal=%s", mode),
	}
}

type CacheMode string

const (
	CacheModeShared CacheMode = "shared"
)

func WithCache(mode CacheMode) Option {
	return &indexPathOption{
		extraSqliteArgs: fmt.Sprintf("cache=%s", mode),
	}
}

type SynchronousMode string

const (
	SynchronousModeOff    SynchronousMode = "OFF"
	SynchronousModeNormal SynchronousMode = "NORMAL"
	SynchronousModeFull   SynchronousMode = "FULL"
	SynchronousModeExtra  SynchronousMode = "EXTRA"
)

func WithSynchronous(mode SynchronousMode) Option {
	return &indexPathOption{
		extraSqliteArgs: fmt.Sprintf("_synchronous=%s", mode),
	}
}

func (o *indexPathOption) apply(config *Config) {
	if o.project != "" {
		config.project = o.project
	}

	if o.indexPath != "" {
		config.indexPath = o.indexPath
	}

	if o.extraSqliteArgs != "" {
		if config.extraSqliteArgs != "" {
			config.extraSqliteArgs += "&"
		}
		config.extraSqliteArgs += o.extraSqliteArgs
	}
}

type recreateOption struct {
	recreate bool
}

func WithRecreate(recreate bool) Option {
	return &recreateOption{
		recreate: recreate,
	}
}

func (o *recreateOption) apply(config *Config) {
	config.recreate = o.recreate
}

type IndexItem interface {
	handle(index *SqlIndex) (interface{}, error)
	respChan() chan IndexItemResp
}

func Handle(item IndexItem, index *SqlIndex) {
	resp, err := item.handle(index)
	respChan := item.respChan()
	if respChan != nil {
		respChan <- IndexItemResp{
			val: resp,
			err: err,
		}
	}
}

type IndexItemResp struct {
	val interface{}
	err error
}

type IndexItemWithResp struct {
	resp chan IndexItemResp
}

func NewIndexItemWithResp() IndexItemWithResp {
	return IndexItemWithResp{
		resp: make(chan IndexItemResp),
	}
}

func (i *IndexItemWithResp) respChan() chan IndexItemResp {
	return i.resp
}

type IndexItemNoResp struct {
}

func NewIndexItemNoResp() IndexItemNoResp {
	return IndexItemNoResp{}
}

func (i *IndexItemNoResp) respChan() chan IndexItemResp {
	return nil
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

func (id DeclarationId) String() string {
	return strconv.Itoa(int(id))
}

type SymbolResp struct {
	symbolId      SymbolId
	declarationId DeclarationId
}

type Index interface {
	AddModule(moduleName string) (ModuleId, error)
	AddFile(moduleId ModuleId, pkgDir, path string, contents []byte) (FileId, error)

	AddSymbol(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType, start token.Position, end token.Position) (SymbolId, DeclarationId, error)
	FindSymbolId(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType) (SymbolId, DeclarationId, error)
	FindDeclarationId(name string, scope SymbolScope, symbolType SymbolType) ([]DeclarationId, error)
	GetAllSymbols() ([]SymbolDecl, error)

	AddReference(from FileId, to token.Position, toName string, start token.Position, end token.Position)
	ResolveReferences() error
	GetAllReferencesNames() ([]*ReferenceNames, error)

	FindBuiltinDeclaration(name string) (FileId, token.Position, token.Position, error)
	FindFileId(path string) (FileId, error)

	Wait() error
	Close() error
}
