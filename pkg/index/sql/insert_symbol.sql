INSERT INTO symbols(name, module_id, symbol_scope)
VALUES (?1, ?2, ?3)
RETURNING id