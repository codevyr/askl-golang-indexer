package index

import (
	"database/sql"
	_ "embed"
	"fmt"
	"go/token"
	"log"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

var (

	//go:embed sql/select_module.sql
	selectModuleSQL string

	//go:embed sql/insert_module.sql
	insertModuleSQL string

	//go:embed sql/select_file.sql
	selectFileSQL string

	//go:embed sql/insert_file.sql
	insertFileSQL string

	//go:embed sql/select_symbol.sql
	selectSymbolSQL string

	//go:embed sql/insert_symbol.sql
	insertSymbolSQL string

	//go:embed sql/select_declaration.sql
	selectDeclarationSQL string

	//go:embed sql/insert_declaration.sql
	insertDeclarationSQL string

	//go:embed sql/insert_reference.sql
	insertReferenceSQL string
)

type ModuleId int64

type ModuleResp struct {
	moduleId ModuleId
}

type Module struct {
	IndexItemWithResp
	name string
}

var _ IndexItem = &File{}

func (f *Module) handle(index *SqlIndex) (interface{}, error) {
	row := index.db.QueryRow(selectModuleSQL, f.name)

	moduleResp := ModuleResp{}
	var err error
	if err = row.Scan(&moduleResp.moduleId); err == nil {
		return moduleResp, nil
	} else if err == sql.ErrNoRows {
		// We exit the if condition to to insert the row.
	} else {
		return nil, err
	}

	res, err := index.db.Exec(insertModuleSQL, f.name)
	if err != nil {
		return nil, err
	}

	var moduleId int64
	if moduleId, err = res.LastInsertId(); err != nil {
		return nil, err
	}

	return ModuleResp{
		moduleId: ModuleId(moduleId),
	}, nil
}

type FileId int64

type FileResp struct {
	fileId FileId
}

type File struct {
	IndexItemWithResp
	module ModuleId
	pkgDir string
	path   string
}

var _ IndexItem = &File{}

func (f *File) handle(index *SqlIndex) (interface{}, error) {
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

	modulePath, ok := strings.CutPrefix(f.path, f.pkgDir)
	if !ok {
		log.Printf("file %v is not in the directory %v", f.path, f.pkgDir)
		modulePath = f.path
	}
	res, err := index.db.Exec(insertFileSQL, f.module, modulePath, f.path, goFileType)
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

type Symbol struct {
	IndexItemWithResp
	moduleId ModuleId
	fileId   FileId
	name     string
	scope    SymbolScope
	start    token.Position
	end      token.Position
}

var _ IndexItem = &Symbol{}

func (s *Symbol) getSymbolId(index *SqlIndex) (SymbolId, error) {

	row := index.db.QueryRow(selectSymbolSQL, s.name, s.moduleId, s.scope)

	var symbolId SymbolId
	var err error
	if err = row.Scan(&symbolId); err == nil {
		return symbolId, nil
	} else if err == sql.ErrNoRows {
		// We exit the if condition to to insert the row.
	} else {
		return -1, err
	}

	res, err := index.db.Exec(insertSymbolSQL, s.name, s.moduleId, s.scope)
	if err != nil {
		return -1, err
	}

	var symbolIdInt int64
	if symbolIdInt, err = res.LastInsertId(); err != nil {
		return -1, err
	}

	return SymbolId(symbolIdInt), nil
}

func (s *Symbol) handle(index *SqlIndex) (interface{}, error) {
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
	from   DeclarationId
	to     token.Position
	toName string
	start  token.Position
	end    token.Position
	wg     *sync.WaitGroup
}

type ReferenceResp struct {
}

var _ IndexItem = &Reference{}

func (i *Reference) handle(index *SqlIndex) (interface{}, error) {
	// First find symbol ID for the `to` symbol.

	defer i.wg.Done()

	row := index.db.QueryRow(
		`SELECT declarations.symbol
FROM ((declarations
INNER JOIN files ON files.id = declarations.file_id)
INNER JOIN symbols ON symbols.id = declarations.symbol)
WHERE files.filesystem_path = ?  AND name = ?`,
		i.to.Filename,
		i.toName,
	)

	var to SymbolId
	if err := row.Scan(&to); err == nil {
		log.Printf("Found symbol %+v", to)
	} else {
		log.Printf("Not Found symbol from=%s '%s' %s-%s %s %s",
			i.from,
			i.to, i.start, i.end, i.toName, err)
		return nil, err
	}

	_, err := index.db.Exec(insertReferenceSQL,
		i.from, to,
		i.start.Line, i.start.Column,
		i.end.Column,
	)
	if err != nil {
		return -1, err
	}

	return ReferenceResp{}, nil
}

type SqlIndex struct {
	project string
	db      *sql.DB
	channel chan IndexItem
	wg      sync.WaitGroup

	referencesLog []*Reference
}

var _ Index = &SqlIndex{}

//go:embed sql/create_tables.sql
var crateTablesSQL string

func NewSqlIndex(options ...Option) (Index, error) {
	var config Config
	for _, option := range options {
		option.apply(&config)
	}

	db, err := sql.Open(
		"sqlite3",
		fmt.Sprintf("%s?%s", config.indexPath, config.extraSqliteArgs),
	)
	if err != nil {
		return nil, err
	}

	if config.recreate {
		_, err = db.Exec(crateTablesSQL)
		if err != nil {
			return nil, err
		}
	}

	index := &SqlIndex{
		project:       config.project,
		db:            db,
		channel:       make(chan IndexItem),
		referencesLog: make([]*Reference, 0),
	}

	go index.loop()

	return index, nil
}

func (i *SqlIndex) AddModule(moduleName string) (ModuleId, error) {
	i.wg.Add(1)

	f := &Module{
		IndexItemWithResp: NewIndexItemWithResp(),
		name:              moduleName,
	}
	i.channel <- f
	resp := <-f.respChan()
	moduleResp, _ := resp.val.(ModuleResp)

	return moduleResp.moduleId, resp.err
}

func (i *SqlIndex) AddFile(moduleId ModuleId, pkgDir, path string) (FileId, error) {
	i.wg.Add(1)

	f := &File{
		IndexItemWithResp: NewIndexItemWithResp(),
		module:            moduleId,
		pkgDir:            pkgDir,
		path:              path,
	}
	i.channel <- f
	resp := <-f.respChan()
	fileResp, _ := resp.val.(FileResp)

	return fileResp.fileId, resp.err
}

func (i *SqlIndex) AddSymbol(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, start token.Position, end token.Position) (SymbolId, DeclarationId, error) {
	i.wg.Add(1)

	log.Printf("AddSymbol: moduleId=%d, fileId=%d, name=%s, scope=%s, start=%v, end=%v",
		moduleId, fileId, name, scope, start, end)
	s := &Symbol{
		IndexItemWithResp: NewIndexItemWithResp(),
		fileId:            fileId,
		moduleId:          moduleId,
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

func (i *SqlIndex) GetAllSymbols() ([]Symbol, error) {
	rows, err := i.db.Query(
		`SELECT symbols.id, symbols.name, symbols.symbol_scope, declarations.file_id, declarations.line_start, declarations.col_start, declarations.line_end, declarations.col_end
FROM ((declarations
INNER JOIN files ON files.id = declarations.file_id)
INNER JOIN symbols ON symbols.id = declarations.symbol)`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	log.Printf("GetAllSymbols: Querying all symbols: %v", i.db.Stats())

	var symbols []Symbol
	for rows.Next() {
		var symbol Symbol
		var startLine, startColumn, endLine, endColumn int
		if err := rows.Scan(&symbol.moduleId, &symbol.name, &symbol.scope, &symbol.fileId, &startLine, &startColumn, &endLine, &endColumn); err != nil {
			return nil, err
		}
		symbol.start = token.Position{Line: startLine, Column: startColumn}
		symbol.end = token.Position{Line: endLine, Column: endColumn}
		symbols = append(symbols, symbol)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return symbols, nil
}

func (i *SqlIndex) AddReference(from DeclarationId, to token.Position, toName string, start token.Position, end token.Position) {
	i.referencesLog = append(i.referencesLog,
		&Reference{
			IndexItemNoResp: NewIndexItemNoResp(),
			from:            from,
			to:              to,
			toName:          toName,
			start:           start,
			end:             end,
			wg:              &i.wg,
		},
	)
}

func (i *SqlIndex) ResolveReferences() error {
	i.wg.Add(len(i.referencesLog) * 2)
	for _, ref := range i.referencesLog {
		go func() {
			i.channel <- ref
		}()
	}

	return nil
}

func (i *SqlIndex) loop() {
	for message := range i.channel {
		Handle(message, i)
		i.wg.Done()
	}
}

func (i *SqlIndex) Close() {
	if i == nil {
		return
	}

	i.wg.Wait()
	close(i.channel)

	i.db.Close()
}
