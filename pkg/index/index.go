package index

import (
	"database/sql"
	_ "embed"
	"fmt"
	"go/token"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

type Config struct {
	indexPath string
	recreate  bool
}

type Option interface {
	apply(config *Config)
}

type indexPathOption struct {
	indexPath string
}

func WithIndexPath(indexPath string) Option {
	return &indexPathOption{
		indexPath: indexPath,
	}
}

func (o *indexPathOption) apply(config *Config) {
	config.indexPath = o.indexPath
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
	handle() error
}

type FileId int

type FileResp struct {
	fileId FileId
	err    error
}

type File struct {
	path string
	resp chan FileResp
}

var _ IndexItem = &File{}

func (i *File) handle() error {
	fmt.Println("GoFiles:", i.path)

	i.resp <- FileResp{
		fileId: 1,
		err:    nil,
	}
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
	case ScopeDeclaration:
		return "declaration"
	case ScopeDefinition:
		return "definition"
	default:
		return "unknown"
	}
}

const (
	ScopeDeclaration SymbolType = iota
	ScopeDefinition
)

type SymbolId int
type DeclarationId int

type SymbolResp struct {
	symbolId      SymbolId
	declarationId DeclarationId
	err           error
}

type Symbol struct {
	fileId FileId
	name   string
	scope  SymbolScope
	start  token.Position
	end    token.Position
	resp   chan SymbolResp
}

var _ IndexItem = &Symbol{}

func (i *Symbol) handle() error {
	fmt.Println("  Symbol:", i.fileId, i.name, i.scope, i.start, i.end)

	i.resp <- SymbolResp{
		symbolId:      1,
		declarationId: 1,
		err:           nil,
	}
	return nil
}

type Reference struct {
	from  DeclarationId
	to    SymbolId
	start token.Position
	end   token.Position
}

var _ IndexItem = &Reference{}

func (i *Reference) handle() error {
	fmt.Println("  Reference:", i.from, i.to, i.start, i.end)

	return nil
}

type Index struct {
	db      *sql.DB
	channel chan IndexItem
	wg      sync.WaitGroup
}

//go:embed sql/create_tables.sql
var crateTablesSQL string

func NewIndex(options ...Option) (*Index, error) {
	var config Config
	for _, option := range options {
		option.apply(&config)
	}

	db, err := sql.Open("sqlite3", config.indexPath)
	if err != nil {
		return nil, err
	}

	if config.recreate {
		_, err = db.Exec(crateTablesSQL)
		if err != nil {
			return nil, err
		}
	}

	index := &Index{
		db:      db,
		channel: make(chan IndexItem),
	}

	go index.loop()

	return index, nil
}

func (i *Index) AddFile(path string) (FileId, error) {
	i.wg.Add(1)

	respChan := make(chan FileResp)
	i.channel <- &File{
		path: path,
		resp: respChan,
	}
	resp := <-respChan

	return resp.fileId, resp.err
}

func (i *Index) AddSymbol(fileId FileId, name string, scope SymbolScope, start token.Position, end token.Position) (SymbolId, DeclarationId, error) {
	i.wg.Add(1)

	respChan := make(chan SymbolResp)
	i.channel <- &Symbol{
		fileId: fileId,
		name:   name,
		scope:  scope,
		start:  start,
		end:    end,
		resp:   respChan,
	}
	resp := <-respChan

	return resp.symbolId, resp.declarationId, resp.err
}

func (i *Index) AddReference(from DeclarationId, to string, start token.Position, end token.Position) {
	i.wg.Add(1)

	go func() {
		i.channel <- &Reference{
			from:  from,
			to:    1,
			start: start,
			end:   end,
		}
	}()
}

func (i *Index) loop() {
	for message := range i.channel {
		message.handle()
		i.wg.Done()
	}
}

func (i *Index) Close() {
	if i == nil {
		return
	}

	i.wg.Wait()
	close(i.channel)

	i.db.Close()
}
