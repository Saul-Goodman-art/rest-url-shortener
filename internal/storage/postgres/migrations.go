package postgres

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // Драйвер БД для миграций
	_ "github.com/golang-migrate/migrate/v4/source/file"       // Драйвер чтения из папки
)

// RunMigrations применяет все новые миграции к базе данных.
// migrationsPath - путь к папке с файлами миграций (например, "file://migrations")
func RunMigrations(dsn string, migrationsPath string) error {
	const op = "storage.postgres.RunMigrations"

	// golang-migrate требует немного другой формат DSN (с параметром sslmode)
	m, err := migrate.New(
		migrationsPath,
		dsn,
	)
	if err != nil {
		return fmt.Errorf("%s: failed to create migrate instance: %w", op, err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		// ErrNoChange не является ошибкой — это значит, что миграции уже применены и база в актуальном состоянии
		return fmt.Errorf("%s: failed to apply migrations: %w", op, err)
	}

	return nil
}
