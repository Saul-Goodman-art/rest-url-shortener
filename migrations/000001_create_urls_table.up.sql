CREATE TABLE IF NOT EXISTS urls (
                                    id SERIAL PRIMARY KEY,
                                    alias TEXT NOT NULL UNIQUE,
                                    url TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_alias ON urls(alias);