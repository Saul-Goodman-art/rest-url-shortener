package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"   // <--- ДОБАВИТЬ ЭТО
	_ "github.com/jackc/pgx/v5/stdlib" // Драйвер БД
	"url-shortener/internal/storage"
)

type Storage struct {
	db *sql.DB
}

// New создает новый экземпляр Storage и проверяет подключение к БД.
// Также здесь создается таблица, если её не было (для удобства, в реальном проде лучше использовать миграции).
func New(dsn string) (*Storage, error) {
	const op = "storage.postgres.New"

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	// Проверяем, что подключение реально работает
	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("%s: ping error: %w", op, err)
	}

	// Инициализируем таблицу
	if err := initTable(db); err != nil {
		return nil, fmt.Errorf("%s: init table error: %w", op, err)
	}

	return &Storage{db: db}, nil
}

func initTable(db *sql.DB) error {
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS urls (
            id SERIAL PRIMARY KEY,
            alias TEXT NOT NULL UNIQUE,
            url TEXT NOT NULL
        );
        CREATE UNIQUE INDEX IF NOT EXISTS idx_alias ON urls(alias);
    `)
	return err
}

func (s *Storage) SaveURL(urlToSave string, alias string) (int64, error) {
	const op = "storage.postgres.SaveURL"

	// ВНИМАНИЕ: В Postgres используются плейсхолдеры $1, $2, а не ?, как в SQLite
	stmt, err := s.db.Prepare(`INSERT INTO urls (url, alias) VALUES ($1, $2) RETURNING id`)
	if err != nil {
		return 0, fmt.Errorf("%s: prepare statement: %w", op, err)
	}
	defer stmt.Close()

	var id int64
	err = stmt.QueryRow(urlToSave, alias).Scan(&id)
	if err != nil {
		// Проверяем ошибку уникального ограничения (код 23505 в Postgres)
		if isUniqueViolationError(err) {
			return 0, fmt.Errorf("%s: %w", op, storage.ErrURLExists)
		}
		return 0, fmt.Errorf("%s: execute statement: %w", op, err)
	}

	return id, nil
}

func (s *Storage) GetURL(alias string) (string, error) {
	const op = "storage.postgres.GetURL"

	stmt, err := s.db.Prepare(`SELECT url FROM urls WHERE alias = $1`)
	if err != nil {
		return "", fmt.Errorf("%s: prepare statement: %w", op, err)
	}
	defer stmt.Close()

	var resURL string
	err = stmt.QueryRow(alias).Scan(&resURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrURLNotFound)
		}
		return "", fmt.Errorf("%s: execute statement: %w", op, err)
	}

	return resURL, nil
}

func (s *Storage) DeleteURL(alias string) (string, error) {
	const op = "storage.postgres.DeleteURL"

	stmt, err := s.db.Prepare(`DELETE FROM urls WHERE alias = $1 RETURNING url`)
	if err != nil {
		return "", fmt.Errorf("%s: prepare statement: %w", op, err)
	}
	defer stmt.Close()

	var deletedURL string
	err = stmt.QueryRow(alias).Scan(&deletedURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrURLNotFound)
		}
		return "", fmt.Errorf("%s: execute statement: %w", op, err)
	}

	return deletedURL, nil
}

// Вспомогательная функция для проверки ошибки уникальности Postgres
// Проверяем по SQLSTATE коду 23505 (unique_violation)
func isUniqueViolationError(err error) bool {
	var pgErr *pgconn.PgError

	// errors.As раскручивает обертки ошибок (которые делает database/sql)
	// и ищет внутри ошибку типа *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	return false
}
