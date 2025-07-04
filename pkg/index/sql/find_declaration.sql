SELECT symbols.id, declarations.id
FROM declarations
INNER JOIN symbols ON declarations.symbol = symbols.id
WHERE symbols.module = ? AND declarations.file_id = ? AND symbols.name = ? AND symbols.symbol_scope = ? AND declarations.symbol_type = ?