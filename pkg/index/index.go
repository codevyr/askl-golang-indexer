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
	handle(index *Index) error
}

type FileId int64

type FileResp struct {
	fileId FileId
	err    error
}

type File struct {
	pkgDir string
	path   string
	resp   chan FileResp
}

var _ IndexItem = &File{}

//go:embed sql/select_file.sql
var selectFileSQL string

//go:embed sql/insert_file.sql
var insertFileSQL string

func (f *File) handle(index *Index) error {
	fmt.Println("GoFiles:", f.path)

	row := index.db.QueryRow(selectFileSQL, f.path, index.project)

	fileResp := FileResp{}
	var err error
	if err = row.Scan(&fileResp.fileId); err == nil {
		f.resp <- fileResp
		return err
	} else if err == sql.ErrNoRows {
		// We exit the if condition to to insert the row.
	} else {
		fileResp.err = err
		f.resp <- fileResp
		return err
	}

	res, err := index.db.Exec(insertFileSQL, index.project, f.pkgDir, f.path, goFileType)
	if err != nil {
		fileResp.err = err
		f.resp <- fileResp
		return err
	}

	var fileId int64
	if fileId, err = res.LastInsertId(); err != nil {
		fileResp.err = err
		f.resp <- fileResp
		return err
	}

	f.resp <- FileResp{
		fileId: FileId(fileId),
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

func (i *Symbol) handle(index *Index) error {
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

func (i *Reference) handle(index *Index) error {
	fmt.Println("  Reference:", i.from, i.to, i.start, i.end)

	return nil
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

	respChan := make(chan FileResp)
	i.channel <- &File{
		pkgDir: pkgDir,
		path:   path,
		resp:   respChan,
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
		message.handle(i)
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
