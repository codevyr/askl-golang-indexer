INSERT INTO symbols(name, module, symbol_scope)
VALUES (?1, ?2, ?3)
RETURNING id