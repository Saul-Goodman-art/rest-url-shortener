package postgres_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"url-shortener/internal/storage"
	pgstore "url-shortener/internal/storage/postgres"
)

func setupTestDB(t *testing.T) (*pgstore.Storage, func()) {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("url_shortener_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = ctr.Terminate(ctx)
	})

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	dsn = strings.ReplaceAll(dsn, "localhost", "127.0.0.1")

	absMigrationsPath, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations"))
	require.NoError(t, err)
	migrationsURL := "file://" + filepath.ToSlash(absMigrationsPath)

	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer db.Close()

	err = db.Ping()
	require.NoError(t, err)

	err = pgstore.RunMigrations(dsn, migrationsURL)
	require.NoError(t, err)

	s, err := pgstore.New(dsn)
	require.NoError(t, err)
	return s, func() {}
}

func TestStorage_Save_Get_Delete(t *testing.T) {
	s, _ := setupTestDB(t)

	t.Run("Save and Get", func(t *testing.T) {

		id, err := s.SaveURL("https://google.com", "google_test")
		require.NoError(t, err)
		assert.Greater(t, id, int64(0))

		got, err := s.GetURL("google_test")
		require.NoError(t, err)
		assert.Equal(t, "https://google.com", got)
	})

	t.Run("Delete", func(t *testing.T) {
		_, err := s.SaveURL("https://to-delete.example", "to_delete")
		require.NoError(t, err)

		deletedURL, err := s.DeleteURL("to_delete")
		require.NoError(t, err)
		assert.Equal(t, "https://to-delete.example", deletedURL)

		_, err = s.GetURL("to_delete")
		require.ErrorIs(t, err, storage.ErrURLNotFound)
	})
}
