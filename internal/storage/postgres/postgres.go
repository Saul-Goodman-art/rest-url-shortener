package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"url-shortener/internal/storage"
)

type Storage struct {
	db *sql.DB
}

func New(dsn string) (*Storage, error) {
	const op = "storage.postgres.New"

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("%s: ping error: %w", op, err)
	}
	return &Storage{db: db}, nil
}

func (s *Storage) SaveURL(urlToSave string, alias string) (int64, error) {
	const op = "storage.postgres.SaveURL"
	stmt, err := s.db.Prepare(`INSERT INTO urls (url, alias) VALUES ($1, $2) RETURNING id`)
	if err != nil {
		return 0, fmt.Errorf("%s: prepare statement: %w", op, err)
	}
	defer stmt.Close()

	var id int64
	err = stmt.QueryRow(urlToSave, alias).Scan(&id)
	if err != nil {
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

func isUniqueViolationError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
