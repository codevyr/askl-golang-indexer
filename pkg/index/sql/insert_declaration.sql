INSERT INTO declarations (symbol, file_id, symbol_type, line_start, col_start, line_end, col_end)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id