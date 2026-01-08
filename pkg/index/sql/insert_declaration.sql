INSERT INTO declarations (symbol, file_id, symbol_type, start_offset, end_offset)
VALUES (?, ?, ?, ?, ?)
RETURNING id
