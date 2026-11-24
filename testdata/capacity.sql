-- Synthetic sizing fixture: incompressible rows plus compressible large text.
-- Source data is disposable; no real customer rows or credentials are used.
CREATE TABLE payloads AS
SELECT i AS id, string_agg(md5(i::text || ':' || g::text), '' ORDER BY g) AS payload
FROM generate_series(1,100000) i CROSS JOIN generate_series(1,32) g GROUP BY i;
ALTER TABLE payloads ADD PRIMARY KEY (id);
CREATE TABLE expansion_probe AS
SELECT i AS id, repeat(md5('expansion:' || i::text),35000) AS payload
FROM generate_series(1,128) i;
