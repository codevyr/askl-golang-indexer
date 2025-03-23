package index

import (
	"database/sql"
	_ "embed"
	"fmt"
	"go/token"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

const (
	goFileType = "go"
)

type Config struct {
	project   string
	indexPath string
	recreate  bool
}

type Option interface {
	apply(config *Config)
}

type indexPathOption struct {
	indexPath string
	project   string
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
	handle(index *Index) (interface{}, error)
	respChan() chan IndexItemResp
}

func Handle(item IndexItem, index *Index) {
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

type FileId int64

type FileResp struct {
	fileId FileId
}

type File struct {
	IndexItemWithResp
	pkgDir string
	path   string
}

var _ IndexItem = &File{}

//go:embed sql/select_file.sql
var selectFileSQL string

//go:embed sql/insert_file.sql
var insertFileSQL string

func (f *File) handle(index *Index) (interface{}, error) {
	fmt.Println("GoFiles:", f.path)

	row := index.db.QueryRow(selectFileSQL, f.path, index.project)

	fileResp := FileResp{}
	var err error
	if err = row.Scan(&fileResp.fileId); err == nil {
		return fileResp, nil
	} else if err == sql.ErrNoRows {
		// We exit the if condition to to insert the row.
	} else {
		return nil, err
	}

	res, err := index.db.Exec(insertFileSQL, index.project, f.pkgDir, f.path, goFileType)
	if err != nil {
		return nil, err
	}

	var fileId int64
	if fileId, err = res.LastInsertId(); err != nil {
		return nil, err
	}

	return FileResp{
		fileId: FileId(fileId),
	}, nil
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
}

type Symbol struct {
	IndexItemWithResp
	fileId FileId
	name   string
	scope  SymbolScope
	start  token.Position
	end    token.Position
}

var _ IndexItem = &Symbol{}

//go:embed sql/select_symbol.sql
var selectSymbolSQL string

//go:embed sql/insert_symbol.sql
var insertSymbolSQL string

//go:embed sql/select_declaration.sql
var selectDeclarationSQL string

//go:embed sql/insert_declaration.sql
var insertDeclarationSQL string

func (s *Symbol) getSymbolId(index *Index) (SymbolId, error) {

	row := index.db.QueryRow(selectSymbolSQL, s.name, s.fileId, s.scope)

	var symbolId SymbolId
	var err error
	if err = row.Scan(&symbolId); err == nil {
		return symbolId, nil
	} else if err == sql.ErrNoRows {
		// We exit the if condition to to insert the row.
	} else {
		return -1, err
	}

	res, err := index.db.Exec(insertSymbolSQL, s.name, s.fileId, s.scope)
	if err != nil {
		return -1, err
	}

	var symbolIdInt int64
	if symbolIdInt, err = res.LastInsertId(); err != nil {
		return -1, err
	}

	return SymbolId(symbolIdInt), nil
}

func (s *Symbol) handle(index *Index) (interface{}, error) {
	fmt.Println("  Symbol:", s.fileId, s.name, s.scope, s.start, s.end)

	symbolId, err := s.getSymbolId(index)
	if err != nil {
		return nil, err
	}

	row := index.db.QueryRow(
		selectDeclarationSQL, symbolId, s.fileId,
		s.start.Line, s.start.Column,
		s.end.Line, s.end.Column,
	)

	var declarationId DeclarationId
	if err = row.Scan(&declarationId); err == nil {
		return SymbolResp{
			symbolId:      symbolId,
			declarationId: declarationId,
		}, nil
	} else if err == sql.ErrNoRows {
		// We exit the if condition to to insert the row.
	} else {
		return -1, err
	}

	res, err := index.db.Exec(insertDeclarationSQL,
		symbolId, s.fileId,
		ScopeDefinition,
		s.start.Line, s.start.Column,
		s.end.Line, s.end.Column,
	)
	if err != nil {
		return -1, err
	}

	var declarationIdInt int64
	if declarationIdInt, err = res.LastInsertId(); err != nil {
		return -1, err
	}

	return SymbolResp{
		symbolId:      symbolId,
		declarationId: DeclarationId(declarationIdInt),
	}, nil
}

type Reference struct {
	IndexItemNoResp
	from  DeclarationId
	to    SymbolId
	start token.Position
	end   token.Position
}

var _ IndexItem = &Reference{}

func (i *Reference) handle(index *Index) (interface{}, error) {
	fmt.Println("  Reference:", i.from, i.to, i.start, i.end)

	return nil, nil
}

type Index struct {
	project string
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
		project: config.project,
		db:      db,
		channel: make(chan IndexItem),
	}

	go index.loop()

	return index, nil
}

func (i *Index) AddFile(pkgDir, path string) (FileId, error) {
	i.wg.Add(1)

	f := &File{
		IndexItemWithResp: NewIndexItemWithResp(),
		pkgDir:            pkgDir,
		path:              path,
	}
	i.channel <- f
	resp := <-f.respChan()
	fileResp := resp.val.(FileResp)

	return fileResp.fileId, resp.err
}

func (i *Index) AddSymbol(fileId FileId, name string, scope SymbolScope, start token.Position, end token.Position) (SymbolId, DeclarationId, error) {
	i.wg.Add(1)

	s := &Symbol{
		IndexItemWithResp: NewIndexItemWithResp(),
		fileId:            fileId,
		name:              name,
		scope:             scope,
		start:             start,
		end:               end,
	}
	i.channel <- s
	resp := <-s.respChan()
	if resp.err != nil {
		return -1, -1, resp.err
	}
	symResp := resp.val.(SymbolResp)

	return symResp.symbolId, symResp.declarationId, resp.err
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
		Handle(message, i)
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
