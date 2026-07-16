package postgres

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(dsn string, migrationsPath string) error {
	const op = "storage.postgres.RunMigrations"

	m, err := migrate.New(
		migrationsPath,
		dsn,
	)
	if err != nil {
		return fmt.Errorf("%s: failed to create migrate instance: %w", op, err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("%s: failed to apply migrations: %w", op, err)
	}
	return nil
}
