package index

import (
	"cmp"
	"database/sql"
	_ "embed"
	"fmt"
	"go/token"
	"log"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	"github.com/onsi/gomega/types"
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

	//go:embed sql/find_declaration.sql
	findDeclarationSQL string

	//go:embed sql/insert_declaration.sql
	insertDeclarationSQL string

	//go:embed sql/insert_reference.sql
	insertReferenceSQL string
)

type ModuleId int64

func (m ModuleId) Compare(other ModuleId) int {
	return cmp.Compare(int64(m), int64(other))
}

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

func (f FileId) Compare(other FileId) int {
	return cmp.Compare(int64(f), int64(other))
}

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
	modulePath, ok := strings.CutPrefix(f.path, f.pkgDir)
	if !ok {
		log.Printf("file %v is not in the directory %v", f.path, f.pkgDir)
		modulePath = f.path
	}

	row := index.db.QueryRow(selectFileSQL, f.module, modulePath)

	fileResp := FileResp{}
	var err error
	if err = row.Scan(&fileResp.fileId); err == nil {
		return fileResp, nil
	} else if err == sql.ErrNoRows {
		// We exit the if condition to to insert the row.
	} else {
		return nil, err
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
	moduleId   ModuleId
	fileId     FileId
	name       string
	scope      SymbolScope
	symbolType SymbolType
	start      token.Position
	end        token.Position
}

var _ IndexItem = &Symbol{}

func NewSymbol(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType, start *token.Position, end *token.Position) *Symbol {
	s := &Symbol{
		IndexItemWithResp: NewIndexItemWithResp(),
		moduleId:          moduleId,
		fileId:            fileId,
		name:              name,
		scope:             scope,
		symbolType:        symbolType,
	}
	if start != nil {
		s.start = *start
	} else {
		s.start = token.Position{Line: 0, Column: 0}
	}
	if end != nil {
		s.end = *end
	} else {
		s.end = token.Position{Line: 0, Column: 0}
	}
	return s
}

func (s *Symbol) Compare(other *Symbol) int {
	if s.moduleId != other.moduleId {
		return s.moduleId.Compare(other.moduleId)
	}
	if s.fileId != other.fileId {
		return s.fileId.Compare(other.fileId)
	}
	if s.name != other.name {
		return strings.Compare(s.name, other.name)
	}
	if s.scope != other.scope {
		return cmp.Compare(s.scope, other.scope)
	}
	return 0
}

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
		s.symbolType,
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

type SymbolMatcher struct {
	Expected *Symbol
}

func RepresentSymbol(expected *Symbol) types.GomegaMatcher {
	return &SymbolMatcher{
		Expected: expected,
	}
}

func (matcher *SymbolMatcher) Match(actual any) (success bool, err error) {
	s, ok := actual.(Symbol)
	if !ok {
		return false, fmt.Errorf("SymbolMatcher matcher expects a Symbol, got %T", actual)
	}

	rest := strings.HasSuffix(s.name, matcher.Expected.name) &&
		s.scope == matcher.Expected.scope
	if !rest {
		return false, nil
	}

	zeroPosition := token.Position{Line: 0, Column: 0}
	if matcher.Expected.start == zeroPosition && matcher.Expected.end == zeroPosition {
		return true, nil
	}

	return s.start == matcher.Expected.start &&
		s.end == matcher.Expected.end, nil
}

func (matcher *SymbolMatcher) FailureMessage(actual any) (message string) {
	var actualString string
	if s, ok := actual.(Symbol); ok {
		actualString = fmt.Sprintf("{\n\tmoduleId: %d,\n\tfileId: %d,\n\tname: %s,\n\tscope: %s,\n\tstart: %v,\n\tend: %v\n}",
			s.moduleId, s.fileId, s.name, s.scope, s.start, s.end)
	} else {
		actualString = fmt.Sprintf("%#v", actual)
	}

	var expectedString string
	if matcher.Expected != nil {
		expectedString = fmt.Sprintf("{\n\tmoduleId: %d,\n\tfileId: %d,\n\tname: %s,\n\tscope: %s,\n\tstart: %v,\n\tend: %v\n}",
			matcher.Expected.moduleId, matcher.Expected.fileId, matcher.Expected.name,
			matcher.Expected.scope, matcher.Expected.start, matcher.Expected.end)
	} else {
		expectedString = "nil"
	}
	return fmt.Sprintf("Expected\n\t%s\nto contain the Symbol representation of\n\t%s", actualString, expectedString)
}

func (matcher *SymbolMatcher) NegatedFailureMessage(actual any) (message string) {
	var actualString string
	if s, ok := actual.(Symbol); ok {
		actualString = fmt.Sprintf("{\n\tmoduleId: %d,\n\tfileId: %d,\n\tname: %s,\n\tscope: %s,\n\tstart: %v,\n\tend: %v\n}",
			s.moduleId, s.fileId, s.name, s.scope, s.start, s.end)
	} else {
		actualString = fmt.Sprintf("%#v", actual)
	}

	var expectedString string
	if matcher.Expected != nil {
		expectedString = fmt.Sprintf("{\n\tmoduleId: %d,\n\tfileId: %d,\n\tname: %s,\n\tscope: %s,\n\tstart: %v,\n\tend: %v\n}",
			matcher.Expected.moduleId, matcher.Expected.fileId, matcher.Expected.name,
			matcher.Expected.scope, matcher.Expected.start, matcher.Expected.end)
	} else {
		expectedString = "nil"
	}
	return fmt.Sprintf("Expected\n\t%s\nnot to contain the Symbol representation of\n\t%s", actualString, expectedString)
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
	log.Printf("Adding reference from=%s to=%s '%s' %s-%s %s %s",
		i.from,
		i.to, i.toName, i.start, i.end, i.to.Filename, index.project)

	var to SymbolId
	if err := row.Scan(&to); err != nil {
		log.Printf("Not Found symbol from=%s '%s' %s-%s %s %s",
			i.from,
			i.to, i.start, i.end, i.toName, err)
		return nil, err
	}

	log.Printf("Adding reference from=%s to=%s '%s' %s-%s %s %s",
		i.from,
		i.to, i.toName, i.start, i.end, i.to.Filename, index.project)
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
	mu sync.Mutex

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

func (i *SqlIndex) AddSymbol(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType, start token.Position, end token.Position) (SymbolId, DeclarationId, error) {
	i.wg.Add(1)

	log.Printf("AddSymbol: moduleId=%d, fileId=%d, name=%s, scope=%s, symbolType=%s, start=%v, end=%v",
		moduleId, fileId, name, scope, symbolType, start, end)
	s := &Symbol{
		IndexItemWithResp: NewIndexItemWithResp(),
		fileId:            fileId,
		moduleId:          moduleId,
		name:              name,
		scope:             scope,
		symbolType:        symbolType,
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

func (i *SqlIndex) FindSymbol(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType) (SymbolId, DeclarationId, error) {
	log.Printf("FindSymbol: moduleId=%d, fileId=%d, name=%s, scope=%s, symbolType=%s",
		moduleId, fileId, name, scope, symbolType)
	rows, err := i.db.Query(findDeclarationSQL, moduleId, fileId, name, scope, symbolType)
	if err != nil {
		return SymbolId(-1), DeclarationId(-1), err
	}
	defer rows.Close()

	log.Printf("FindSymbol: found %v", i.db.Stats())

	var results []struct {
		symbolId      SymbolId
		declarationId DeclarationId
	}
	for rows.Next() {
		var symbolId SymbolId
		var declarationId DeclarationId
		if err := rows.Scan(&symbolId, &declarationId); err != nil {
			return SymbolId(-1), DeclarationId(-1), err
		}
		results = append(results, struct {
			symbolId      SymbolId
			declarationId DeclarationId
		}{
			symbolId:      symbolId,
			declarationId: declarationId,
		})
	}
	if err := rows.Err(); err != nil {
		return SymbolId(-1), DeclarationId(-1), err
	}

	if len(results) != 1 {
		log.Printf("FindSymbol: expected 1 symbol, found %d", len(results))
		return SymbolId(-1), DeclarationId(-1), fmt.Errorf("expected 1 symbol, found %d", len(results))
	}

	return results[0].symbolId, results[0].declarationId, nil
}

func (i *SqlIndex) GetAllSymbols() ([]Symbol, error) {
	rows, err := i.db.Query(
		`SELECT symbols.module, symbols.name, symbols.symbol_scope, declarations.file_id, declarations.line_start, declarations.col_start, declarations.line_end, declarations.col_end
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
	i.mu.Lock()
	defer i.mu.Unlock()

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

func (i *SqlIndex) Wait() error {
	i.wg.Wait()
	return nil
}

type ReferenceNames struct {
	From string
	To   string
}

func NewReferenceNames(from, to string) *ReferenceNames {
	return &ReferenceNames{
		From: from,
		To:   to,
	}
}

type ReferenceMatcher struct {
	Expected *ReferenceNames
}

func RepresentReference(expected *ReferenceNames) types.GomegaMatcher {
	return &ReferenceMatcher{
		Expected: expected,
	}
}

func (matcher *ReferenceMatcher) Match(actual any) (success bool, err error) {
	r, ok := actual.(*ReferenceNames)
	if !ok {
		return false, fmt.Errorf("ReferenceMatcher matcher expects a ReferenceNames, got %T", actual)
	}

	if !strings.HasSuffix(r.From, matcher.Expected.From) || !strings.HasSuffix(r.To, matcher.Expected.To) {
		return false, nil
	}
	return true, nil
}

func (matcher *ReferenceMatcher) FailureMessage(actual any) (message string) {
	var actualString string
	if r, ok := actual.(ReferenceNames); ok {
		actualString = fmt.Sprintf("{\n\tFrom: %s,\n\tTo: %s\n}", r.From, r.To)
	} else {
		actualString = fmt.Sprintf("%#v", actual)
	}
	var expectedString string
	if matcher.Expected != nil {
		expectedString = fmt.Sprintf("{\n\tFrom: %s,\n\tTo: %s\n}", matcher.Expected.From, matcher.Expected.To)
	} else {
		expectedString = "nil"
	}
	return fmt.Sprintf("Expected\n\t%s\nto contain the ReferenceNames representation of\n\t%s", actualString, expectedString)
}

func (matcher *ReferenceMatcher) NegatedFailureMessage(actual any) (message string) {
	var actualString string
	if r, ok := actual.(ReferenceNames); ok {
		actualString = fmt.Sprintf("{\n\tFrom: %s,\n\tTo: %s\n}", r.From, r.To)
	} else {
		actualString = fmt.Sprintf("%#v", actual)
	}
	var expectedString string
	if matcher.Expected != nil {
		expectedString = fmt.Sprintf("{\n\tFrom: %s,\n\tTo: %s\n}", matcher.Expected.From, matcher.Expected.To)
	} else {
		expectedString = "nil"
	}
	return fmt.Sprintf("Expected\n\t%s\nnot to contain the ReferenceNames representation of\n\t%s", actualString, expectedString)
}

func (i *SqlIndex) GetAllReferencesNames() ([]*ReferenceNames, error) {
	rows, err := i.db.Query(
		`SELECT 
		   from_symbols.name,
		   to_symbols.name
		 FROM (((symbol_refs
		  INNER JOIN symbols AS to_symbols ON symbol_refs.to_symbol = to_symbols.id)
		   INNER JOIN declarations ON symbol_refs.from_decl = declarations.id)
		    INNER JOIN symbols AS from_symbols ON from_symbols.id = declarations.symbol)`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var references []*ReferenceNames
	for rows.Next() {
		var ref ReferenceNames
		if err := rows.Scan(
			&ref.From,
			&ref.To,
		); err != nil {
			return nil, err
		}
		references = append(references, &ref)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	log.Printf("GetAllReferences: Queried %d references", len(references))
	return references, nil
}

func (i *SqlIndex) loop() {
	for message := range i.channel {
		Handle(message, i)
		i.wg.Done()
	}
}

func (i *SqlIndex) Close() error {
	if i == nil {
		return nil
	}

	i.wg.Wait()
	close(i.channel)

	return i.db.Close()
}
