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

/*
Тест делает следующее:
	Поднимает временный PostgreSQL‑контейнер через Testcontainers.
	Получает DSN и пингует БД. (DSN — это строка подключения к базе данных.)
	Находит папку с миграциями и накатывает их.
	Создаёт экземпляр pgstore.Storage.
	Проверяет:
		сохранение URL,
		получение URL,
		удаление URL.
*/

func setupTestDB(t *testing.T) (*pgstore.Storage, func()) {
	t.Helper()
	ctx := context.Background()

	//Запуск контейнера PostgreSQL
	ctr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("url_shortener_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)

	// Гарантируем очистку контейнера после завершения теста
	t.Cleanup(func() {
		_ = ctr.Terminate(ctx)
	})

	//testcontainers сам формирует строку подключения.
	//
	//На Windows заменяется localhost → 127.0.0.1, чтобы избежать проблем Docker Desktop:
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// На Windows принудительно используем IPv4 (исключает проблемы с Docker)
	dsn = strings.ReplaceAll(dsn, "localhost", "127.0.0.1")

	//Поиск миграций
	// Формируем абсолютный путь к папке с миграциями
	absMigrationsPath, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations"))
	require.NoError(t, err)

	migrationsURL := "file://" + filepath.ToSlash(absMigrationsPath)

	// Пингуем БД, чтобы быть абсолютно уверенным, что контейнер готов принимать запросы
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer db.Close()

	//ping db Это важно: Testcontainers может вернуть DSN до того, как Postgres реально готов принимать запросы.
	//Пинг гарантирует готовность
	err = db.Ping()
	require.NoError(t, err)

	// Накатываем миграции
	err = pgstore.RunMigrations(dsn, migrationsURL)
	require.NoError(t, err)

	// Инициализируем (создаем) наше хранилище. Возвращается готовый объект Storage, который тесты будут использовать.
	s, err := pgstore.New(dsn)
	require.NoError(t, err)

	return s, func() {}
}

func TestStorage_Save_Get_Delete(t *testing.T) {
	s, _ := setupTestDB(t)

	t.Run("Save and Get", func(t *testing.T) {

		// test save + get. Проверяется
		//что запись сохраняется,
		//что ID > 0.
		id, err := s.SaveURL("https://google.com", "google_test")
		require.NoError(t, err)
		assert.Greater(t, id, int64(0))

		//Проверяется корректное чтение
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
