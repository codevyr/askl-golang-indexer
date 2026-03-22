package index

import (
	"crypto/sha512"
	"fmt"
	"go/token"
	"math"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/planetA/askl-golang-indexer/pkg/indexpb"
	"github.com/planetA/askl-golang-indexer/pkg/logging"
)

// symbolKey is now project-scoped (no moduleID)
type symbolKey struct {
	name  string
	scope SymbolScope
}

type fileNameKey struct {
	filePath string
	name     string
}

type declKey struct {
	symbolID int64
	fileID   int64
	start    int
	end      int
}

type declLookupKey struct {
	name       string
	scope      SymbolScope
	symbolType SymbolType
}

type declEntry struct {
	symbolID   int64
	fileID     int64
	symbolType SymbolType
	start      int
	end        int
	declID     DeclarationId
}

type referenceLog struct {
	fromFile FileId
	to       token.Position
	toName   string
	start    token.Position
	end      token.Position
}

// moduleImportLog tracks module-to-module import relationships
type moduleImportLog struct {
	fromModuleID int64
	toModuleName string
	fromFileID   int64
	startOffset  int
	endOffset    int
}

// moduleInfo tracks module data for creating module symbols at finalization
type moduleInfo struct {
	name     string
	symbolID int64   // Set when module symbol is created
	fileIDs  []int64 // Files belonging to this module
}

type ProtoIndex struct {
	mu sync.Mutex

	projectName string
	rootPath    string
	project     *indexpb.Project

	nextModuleID int64
	nextFileID   int64
	nextSymbolID int64
	nextDeclID   int64

	// Module tracking (for creating module symbols)
	modulesByName map[string]int64
	moduleByID    map[int64]*moduleInfo

	fileByID            map[int64]*indexpb.Object
	filePathByID        map[int64]string
	fileIDByPath        map[string]int64
	fileHashByPath      map[string]string
	fileModuleID        map[int64]int64 // Maps fileID -> moduleID for module instance creation
	fileSymbolByObjectID map[int64]int64 // Maps objectID -> file symbol ID

	// Symbols are now project-scoped
	symbolByKey        map[symbolKey]int64
	symbolByID         map[int64]*indexpb.Symbol
	symbolNameByID     map[int64]string
	symbolByFileAndName map[fileNameKey]int64

	declIDByKey   map[declKey]DeclarationId
	declsByFile   map[int64][]declEntry
	declsByLookup map[declLookupKey][]DeclarationId
	referencesLog    []referenceLog
	moduleImportsLog []moduleImportLog
}

var _ Index = &ProtoIndex{}

func NewProtoIndex(options ...Option) (*ProtoIndex, error) {
	var config Config
	for _, option := range options {
		option.apply(&config)
	}

	idx := &ProtoIndex{
		projectName: config.project,
		rootPath:    config.rootPath,
		project: &indexpb.Project{
			ProjectName: config.project,
			RootPath:    config.rootPath,
		},
		nextModuleID:        1,
		nextFileID:          1,
		nextSymbolID:        1,
		nextDeclID:          1,
		modulesByName:       make(map[string]int64),
		moduleByID:          make(map[int64]*moduleInfo),
		fileByID:             make(map[int64]*indexpb.Object),
		filePathByID:         make(map[int64]string),
		fileIDByPath:         make(map[string]int64),
		fileHashByPath:       make(map[string]string),
		fileModuleID:         make(map[int64]int64),
		fileSymbolByObjectID: make(map[int64]int64),
		symbolByKey:         make(map[symbolKey]int64),
		symbolByID:          make(map[int64]*indexpb.Symbol),
		symbolNameByID:      make(map[int64]string),
		symbolByFileAndName: make(map[fileNameKey]int64),
		declIDByKey:         make(map[declKey]DeclarationId),
		declsByFile:         make(map[int64][]declEntry),
		declsByLookup:       make(map[declLookupKey][]DeclarationId),
		referencesLog:       []referenceLog{},
		moduleImportsLog:    []moduleImportLog{},
	}

	return idx, nil
}

func (i *ProtoIndex) Marshal() ([]byte, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Create module symbols before marshaling
	i.createModuleSymbols()

	return proto.Marshal(i.project)
}

func (i *ProtoIndex) Upload() *indexpb.Project {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Create module symbols before uploading
	i.createModuleSymbols()

	return proto.Clone(i.project).(*indexpb.Project)
}

// createModuleSymbols creates MODULE type symbols for each tracked module
// and creates symbol instances covering each file in the module.
// This must be called with the mutex held.
func (i *ProtoIndex) createModuleSymbols() {
	for moduleID, info := range i.moduleByID {
		// Skip if already created
		if info.symbolID != 0 {
			continue
		}

		// Create module symbol (project-scoped, scope=UNSPECIFIED for modules)
		symbolID := i.nextSymbolID
		i.nextSymbolID++

		symbol := &indexpb.Symbol{
			LocalId: symbolID,
			Name:    info.name,
			Scope:   indexpb.SymbolScope_SYMBOL_SCOPE_UNSPECIFIED,
			Type:    indexpb.SymbolType_MODULE,
		}

		i.project.Symbols = append(i.project.Symbols, symbol)
		i.symbolByID[symbolID] = symbol
		i.symbolNameByID[symbolID] = info.name
		info.symbolID = symbolID
		i.moduleByID[moduleID] = info

		// Create module symbol instances for each file in the module
		for _, fileID := range info.fileIDs {
			file := i.fileByID[fileID]
			if file == nil {
				continue
			}

			// Module instance covers entire file
			fileLen := int32(len(file.Content))
			file.SymbolInstances = append(file.SymbolInstances, &indexpb.SymbolInstance{
				SymbolLocalId: symbolID,
				StartOffset:   0,
				EndOffset:     fileLen,
			})
		}
	}

	// Resolve module imports after all module symbols are created
	i.resolveModuleImports()
}

// resolveModuleImports converts logged module imports into symbol references.
// This must be called after createModuleSymbols and with the mutex held.
func (i *ProtoIndex) resolveModuleImports() {
	seen := make(map[refDedupKey]struct{})
	for _, imp := range i.moduleImportsLog {
		// Find the target module's symbol ID by name
		toModuleID, ok := i.modulesByName[imp.toModuleName]
		if !ok {
			// Target module not indexed - skip (could be external dependency)
			logging.Debugf("Module import target not found: %s", imp.toModuleName)
			continue
		}

		toInfo := i.moduleByID[toModuleID]
		if toInfo == nil || toInfo.symbolID == 0 {
			logging.Warnf("Module import target has no symbol: %s", imp.toModuleName)
			continue
		}

		// Get the file where the import appears
		file := i.fileByID[imp.fromFileID]
		if file == nil {
			logging.Warnf("Import source file not found: %d", imp.fromFileID)
			continue
		}

		// Deduplicate
		dedupKey := refDedupKey{
			toSymbolID: toInfo.symbolID,
			fromFile:   imp.fromFileID,
			start:      int32(imp.startOffset),
			end:        int32(imp.endOffset),
		}
		if _, ok := seen[dedupKey]; ok {
			continue
		}
		seen[dedupKey] = struct{}{}

		// Add the reference from the import statement to the module symbol
		file.Refs = append(file.Refs, &indexpb.SymbolRef{
			ToSymbolLocalId: toInfo.symbolID,
			FromOffsetStart: int32(imp.startOffset),
			FromOffsetEnd:   int32(imp.endOffset),
		})
	}

	i.moduleImportsLog = nil
}

func computeHash(contents []byte) string {
	return fmt.Sprintf("%x", sha512.Sum512(contents))
}

func toProtoScope(scope SymbolScope) indexpb.SymbolScope {
	switch scope {
	case ScopeLocal:
		return indexpb.SymbolScope_LOCAL
	case ScopeGlobal:
		return indexpb.SymbolScope_GLOBAL
	default:
		return indexpb.SymbolScope_SYMBOL_SCOPE_UNSPECIFIED
	}
}

func fromProtoScope(scope indexpb.SymbolScope) SymbolScope {
	switch scope {
	case indexpb.SymbolScope_LOCAL:
		return ScopeLocal
	case indexpb.SymbolScope_GLOBAL:
		return ScopeGlobal
	default:
		return 0
	}
}

func toProtoType(symbolType SymbolType) indexpb.SymbolType {
	switch symbolType {
	case SymbolTypeFunction:
		return indexpb.SymbolType_FUNCTION
	case SymbolTypeFile:
		return indexpb.SymbolType_FILE
	case SymbolTypeModule:
		return indexpb.SymbolType_MODULE
	case SymbolTypeDirectory:
		return indexpb.SymbolType_DIRECTORY
	default:
		return indexpb.SymbolType_SYMBOL_TYPE_UNSPECIFIED
	}
}

func (i *ProtoIndex) AddModule(moduleName string) (ModuleId, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if moduleID, ok := i.modulesByName[moduleName]; ok {
		return ModuleId(moduleID), nil
	}

	moduleID := i.nextModuleID
	i.nextModuleID++

	// Track module info (symbol will be created at finalization)
	info := &moduleInfo{
		name:    moduleName,
		fileIDs: []int64{},
	}

	i.modulesByName[moduleName] = moduleID
	i.moduleByID[moduleID] = info

	return ModuleId(moduleID), nil
}

// AddModuleImport logs an import relationship from one module to another.
// The import is recorded with the file and offset positions where the import statement appears.
func (i *ProtoIndex) AddModuleImport(fromModuleId ModuleId, toModuleName string, fromFileId FileId, startOffset, endOffset int) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.moduleImportsLog = append(i.moduleImportsLog, moduleImportLog{
		fromModuleID: int64(fromModuleId),
		toModuleName: toModuleName,
		fromFileID:   int64(fromFileId),
		startOffset:  startOffset,
		endOffset:    endOffset,
	})

	return nil
}

func (i *ProtoIndex) AddFile(moduleId *ModuleId, baseDir, path, filetype string, contents []byte) (FileId, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if moduleId != nil {
		info := i.moduleByID[int64(*moduleId)]
		if info == nil {
			return FileId(-1), fmt.Errorf("module %d not found", *moduleId)
		}
	}

	if fileID, ok := i.fileIDByPath[path]; ok {
		currentHash := computeHash(contents)
		storedHash := i.fileHashByPath[path]
		if currentHash != storedHash {
			return FileId(-1), fmt.Errorf("content hash mismatch for file %v", path)
		}
		return FileId(fileID), nil
	}

	modulePath := path
	if baseDir != "" {
		relPath, ok := strings.CutPrefix(path, baseDir)
		if !ok {
			logging.Warnf("file %v is not in the directory %v", path, baseDir)
		} else {
			modulePath = relPath
		}
	}

	fileID := i.nextFileID
	i.nextFileID++

	// Object no longer has module_id field
	file := &indexpb.Object{
		LocalId:         fileID,
		ModulePath:      modulePath,
		FilesystemPath:  path,
		Filetype:        filetype,
		Content:         contents,
		SymbolInstances: []*indexpb.SymbolInstance{},
		Refs:            []*indexpb.SymbolRef{},
	}

	// Create FILE symbol for this file
	fileSymbolID := i.nextSymbolID
	i.nextSymbolID++

	fileSymbol := &indexpb.Symbol{
		LocalId: fileSymbolID,
		Name:    path, // Use filesystem path as symbol name
		Scope:   indexpb.SymbolScope_SYMBOL_SCOPE_UNSPECIFIED,
		Type:    indexpb.SymbolType_FILE,
	}
	i.project.Symbols = append(i.project.Symbols, fileSymbol)
	i.symbolByID[fileSymbolID] = fileSymbol
	i.symbolNameByID[fileSymbolID] = path

	// Create file symbol instance covering entire file
	fileLen := int32(len(contents))
	file.SymbolInstances = append(file.SymbolInstances, &indexpb.SymbolInstance{
		SymbolLocalId: fileSymbolID,
		StartOffset:   0,
		EndOffset:     fileLen,
	})

	// Track file symbol for later use
	i.fileSymbolByObjectID[fileID] = fileSymbolID

	i.project.Objects = append(i.project.Objects, file)
	fileHash := computeHash(contents)

	i.fileByID[fileID] = file
	i.filePathByID[fileID] = path
	i.fileIDByPath[path] = fileID
	i.fileHashByPath[path] = fileHash

	// Track file->module mapping for module instance creation
	if moduleId != nil {
		i.fileModuleID[fileID] = int64(*moduleId)
		info := i.moduleByID[int64(*moduleId)]
		if info != nil {
			info.fileIDs = append(info.fileIDs, fileID)
		}
	}

	return FileId(fileID), nil
}

func (i *ProtoIndex) AddSymbol(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType, start token.Position, end token.Position) (SymbolId, DeclarationId, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	info := i.moduleByID[int64(moduleId)]
	if info == nil {
		return -1, -1, fmt.Errorf("module %d not found", moduleId)
	}

	file := i.fileByID[int64(fileId)]
	if file == nil {
		return -1, -1, fmt.Errorf("file %d not found", fileId)
	}

	// Symbol key is now project-scoped (no moduleID)
	symKey := symbolKey{name: name, scope: scope}
	symbolID, ok := i.symbolByKey[symKey]
	if !ok {
		symbolID = i.nextSymbolID
		i.nextSymbolID++

		symbol := &indexpb.Symbol{
			LocalId: symbolID,
			Name:    name,
			Scope:   toProtoScope(scope),
			Type:    toProtoType(symbolType),
		}

		// Add symbol directly to project.Symbols (not module.Symbols)
		i.project.Symbols = append(i.project.Symbols, symbol)
		i.symbolByKey[symKey] = symbolID
		i.symbolByID[symbolID] = symbol
		i.symbolNameByID[symbolID] = name
	}

	filePath := i.filePathByID[int64(fileId)]
	if filePath != "" {
		i.symbolByFileAndName[fileNameKey{filePath: filePath, name: name}] = symbolID
	}

	key := declKey{symbolID: symbolID, fileID: int64(fileId), start: start.Offset, end: end.Offset}
	if declID, ok := i.declIDByKey[key]; ok {
		return SymbolId(symbolID), declID, nil
	}

	if symbolType == 0 {
		return -1, -1, fmt.Errorf("SymbolType is not set for symbol %s in file %d", name, fileId)
	}
	if start.Offset > math.MaxInt32 || start.Offset < 0 {
		return -1, -1, fmt.Errorf("start offset out of range for %s", name)
	}
	if end.Offset > math.MaxInt32 || end.Offset < 0 {
		return -1, -1, fmt.Errorf("end offset out of range for %s", name)
	}

	declID := DeclarationId(i.nextDeclID)
	i.nextDeclID++

	file.SymbolInstances = append(file.SymbolInstances, &indexpb.SymbolInstance{
		SymbolLocalId: symbolID,
		StartOffset:   int32(start.Offset),
		EndOffset:     int32(end.Offset),
	})

	entry := declEntry{
		symbolID:   symbolID,
		fileID:     int64(fileId),
		symbolType: symbolType,
		start:      start.Offset,
		end:        end.Offset,
		declID:     declID,
	}
	i.declsByFile[int64(fileId)] = append(i.declsByFile[int64(fileId)], entry)
	i.declIDByKey[key] = declID

	lookupKey := declLookupKey{name: name, scope: scope, symbolType: symbolType}
	i.declsByLookup[lookupKey] = append(i.declsByLookup[lookupKey], declID)

	return SymbolId(symbolID), declID, nil
}

func (i *ProtoIndex) FindDeclarationId(name string, scope SymbolScope, symbolType SymbolType) ([]DeclarationId, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	key := declLookupKey{name: name, scope: scope, symbolType: symbolType}
	decls := i.declsByLookup[key]
	if len(decls) == 0 {
		return []DeclarationId{}, nil
	}
	out := make([]DeclarationId, len(decls))
	copy(out, decls)
	return out, nil
}

func (i *ProtoIndex) FindSymbolId(moduleId ModuleId, fileId FileId, name string, scope SymbolScope, symbolType SymbolType) (SymbolId, DeclarationId, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Symbol key is now project-scoped
	symbolID, ok := i.symbolByKey[symbolKey{name: name, scope: scope}]
	if !ok {
		return SymbolId(-1), DeclarationId(-1), fmt.Errorf("symbol not found")
	}

	decls := i.declsByFile[int64(fileId)]
	var matches []declEntry
	for _, decl := range decls {
		if decl.symbolID == symbolID && decl.symbolType == symbolType {
			matches = append(matches, decl)
		}
	}

	if len(matches) != 1 {
		return SymbolId(-1), DeclarationId(-1), fmt.Errorf("expected 1 symbol, found %d", len(matches))
	}

	return SymbolId(symbolID), matches[0].declID, nil
}

func (i *ProtoIndex) FindFileId(path string) (FileId, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	fileID, ok := i.fileIDByPath[path]
	if !ok {
		return FileId(-1), fmt.Errorf("file not found: %s", path)
	}

	return FileId(fileID), nil
}

func (i *ProtoIndex) GetAllSymbols() ([]SymbolDecl, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	symbols := []SymbolDecl{}
	for fileID, decls := range i.declsByFile {
		moduleID := i.fileModuleID[fileID]
		for _, decl := range decls {
			symbol := i.symbolByID[decl.symbolID]
			if symbol == nil {
				continue
			}
			symbols = append(symbols, SymbolDecl{
				ModuleId:   ModuleId(moduleID),
				FileId:     FileId(fileID),
				Name:       symbol.Name,
				Scope:      fromProtoScope(symbol.Scope),
				SymbolType: decl.symbolType,
				Start:      token.Position{Offset: decl.start},
				End:        token.Position{Offset: decl.end},
			})
		}
	}
	return symbols, nil
}

func (i *ProtoIndex) FindBuiltinDeclaration(name string) (FileId, token.Position, token.Position, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	for fileID, decls := range i.declsByFile {
		filePath := i.filePathByID[fileID]
		if !isBuiltinPath(filePath) {
			continue
		}
		for _, decl := range decls {
			symbol := i.symbolByID[decl.symbolID]
			if symbol == nil {
				continue
			}
			if !strings.Contains(symbol.Name, name) {
				continue
			}

			start := token.Position{Filename: filePath, Offset: decl.start}
			end := token.Position{Filename: filePath, Offset: decl.end}
			return FileId(fileID), start, end, nil
		}
	}

	return FileId(-1), token.Position{}, token.Position{}, fmt.Errorf("builtin declaration not found for %s", name)
}

func (i *ProtoIndex) AddReference(from FileId, to token.Position, toName string, start token.Position, end token.Position) error {
	if start.Filename == "" && start.Offset == 0 && start.Line == 0 && start.Column == 0 {
		return fmt.Errorf("invalid reference start position: from=%d to=%s '%s' %s-%s %s",
			from,
			to, toName, start, end, to.Filename)
	}
	if !to.IsValid() {
		return fmt.Errorf("invalid reference to position: from=%d to=%s '%s' %s-%s %s",
			from,
			to, toName, start, end, to.Filename)
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	i.referencesLog = append(i.referencesLog, referenceLog{
		fromFile: from,
		to:       to,
		toName:   toName,
		start:    start,
		end:      end,
	})

	return nil
}

func (i *ProtoIndex) ResolveReferences() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	seen := make(map[refDedupKey]struct{})
	for _, ref := range i.referencesLog {
		key := fileNameKey{filePath: ref.to.Filename, name: ref.toName}
		toSymbolID, ok := i.symbolByFileAndName[key]
		if !ok {
			if isCReference(ref) {
				continue
			}
			fromFile := i.fileByID[int64(ref.fromFile)]
			return fmt.Errorf("strict reference resolution failed for %s:%d:%d -> %s in %s:%d:%d KEY %+s", fromFile.FilesystemPath, ref.start.Line, ref.start.Column, ref.toName, ref.to.Filename, ref.to.Line, ref.to.Column, key)
		}

		file := i.fileByID[int64(ref.fromFile)]
		if file == nil {
			logging.Warnf("reference from file not found: %d", ref.fromFile)
			continue
		}
		if ref.start.Offset > math.MaxInt32 || ref.start.Offset < 0 {
			logging.Warnf("reference start offset out of range: %d", ref.start.Offset)
			continue
		}
		if ref.end.Offset > math.MaxInt32 || ref.end.Offset < 0 {
			logging.Warnf("reference end offset out of range: %d", ref.end.Offset)
			continue
		}

		dedupKey := refDedupKey{
			toSymbolID: toSymbolID,
			fromFile:   int64(ref.fromFile),
			start:      int32(ref.start.Offset),
			end:        int32(ref.end.Offset),
		}
		if _, ok := seen[dedupKey]; ok {
			continue
		}
		seen[dedupKey] = struct{}{}

		file.Refs = append(file.Refs, &indexpb.SymbolRef{
			ToSymbolLocalId: toSymbolID,
			FromOffsetStart: int32(ref.start.Offset),
			FromOffsetEnd:   int32(ref.end.Offset),
		})
	}

	i.referencesLog = nil

	return nil
}

type refDedupKey struct {
	toSymbolID int64
	fromFile   int64
	start      int32
	end        int32
}

func (i *ProtoIndex) GetAllReferencesNames() ([]*ReferenceNames, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Ensure module symbols and module import refs are created
	i.createModuleSymbols()

	var references []*ReferenceNames
	for fileID, file := range i.fileByID {
		if len(file.Refs) == 0 {
			continue
		}
		decls := i.declsByFile[fileID]

		// Get module name for this file (for module-level refs)
		moduleID := i.fileModuleID[fileID]
		moduleInfo := i.moduleByID[moduleID]
		var moduleName string
		if moduleInfo != nil {
			moduleName = moduleInfo.name
		}

		for _, ref := range file.Refs {
			toName := i.symbolNameByID[ref.ToSymbolLocalId]

			// Check if the target is a module symbol
			toSymbol := i.symbolByID[ref.ToSymbolLocalId]
			isModuleImportRef := toSymbol != nil && toSymbol.Type == indexpb.SymbolType_MODULE

			// Check if this ref is contained in any function declaration
			foundContainingDecl := false
			for _, decl := range decls {
				if decl.start <= int(ref.FromOffsetStart) && decl.end >= int(ref.FromOffsetEnd) {
					fromName := i.symbolNameByID[decl.symbolID]
					references = append(references, &ReferenceNames{
						From: fromName,
						To:   toName,
					})
					foundContainingDecl = true
				}
			}

			// If this is a module import ref and no containing declaration found,
			// attribute it to the module itself
			if isModuleImportRef && !foundContainingDecl && moduleName != "" {
				references = append(references, &ReferenceNames{
					From: moduleName,
					To:   toName,
				})
			}
		}
	}

	return references, nil
}

func (i *ProtoIndex) Wait() error {
	return nil
}

func (i *ProtoIndex) Close() error {
	return nil
}

func isBuiltinPath(path string) bool {
	return strings.HasSuffix(path, "/builtin/builtin.go") ||
		strings.HasSuffix(path, "/src/unsafe/unsafe.go")
}

func isCReference(ref referenceLog) bool {
	return strings.HasPrefix(ref.toName, "C.")
}
