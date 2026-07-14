package postgres_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	//"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"url-shortener/internal/storage"
	"url-shortener/internal/storage/postgres"
)

// setupTestDB создает временный контейнер Postgres, накатывает миграции и возвращает готовое хранилище.
// В конце вызывается teardown для удаления контейнера.
func setupTestDB(t *testing.T) (*postgres.Storage, func()) {
	t.Helper()

	ctx := context.Background()

	// 1. Настраиваем контейнер
	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("url_shortener_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err, "Не удалось запустить контейнер Postgres")

	// Функция очистки (удаление контейнера после теста)
	teardown := func() {
		require.NoError(t, container.Terminate(ctx), "Не удалось остановить контейнер")
	}

	// 2. Получаем строку подключения (DSN) из запущенного контейнера
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Не удалось получить DSN")

	// 3. Ищем папку с миграциями (идем на 2 уровня вверх от текущего файла)
	migrationsPath := "file://" + filepath.Join("..", "..", "..", "migrations")

	// 4. Накатываем миграции в эту тестовую БД
	err = postgres.RunMigrations(dsn, migrationsPath)
	require.NoError(t, err, "Не удалось применить миграции")

	// 5. Создаем наш Storage
	s, err := postgres.New(dsn)
	require.NoError(t, err, "Не удалось инициализировать Storage")

	return s, teardown
}

// ======= САМИ ТЕСТЫ =======

func TestStorage_SaveURL(t *testing.T) {
	s, teardown := setupTestDB(t)
	defer teardown()

	t.Run("Success", func(t *testing.T) {
		id, err := s.SaveURL("https://google.com", "google")
		require.NoError(t, err)
		assert.Greater(t, id, int64(0))
	})

	t.Run("Duplicate Alias", func(t *testing.T) {
		// Сначала сохраняем
		_, err := s.SaveURL("https://ya.ru", "yandex")
		require.NoError(t, err)

		// Пытаемся сохранить с тем же алиасом
		_, err = s.SaveURL("https://ya.ru/new", "yandex")
		require.ErrorIs(t, err, storage.ErrURLExists)
	})
}

func TestStorage_GetURL(t *testing.T) {
	s, teardown := setupTestDB(t)
	defer teardown()

	t.Run("Success", func(t *testing.T) {
		// Подготавливаем данные
		_, err := s.SaveURL("https://github.com", "github")
		require.NoError(t, err)

		// Ищем
		url, err := s.GetURL("github")
		require.NoError(t, err)
		assert.Equal(t, "https://github.com", url)
	})

	t.Run("Not Found", func(t *testing.T) {
		_, err := s.GetURL("nonexistent_alias")
		require.ErrorIs(t, err, storage.ErrURLNotFound)
	})
}

func TestStorage_DeleteURL(t *testing.T) {
	s, teardown := setupTestDB(t)
	defer teardown()

	t.Run("Success", func(t *testing.T) {
		// Подготавливаем данные
		_, err := s.SaveURL("https://test.com", "test_del")
		require.NoError(t, err)

		// Удаляем
		deletedURL, err := s.DeleteURL("test_del")
		require.NoError(t, err)
		assert.Equal(t, "https://test.com", deletedURL)

		// Проверяем, что реально удалилось (при поиске должна быть ошибка)
		_, err = s.GetURL("test_del")
		require.ErrorIs(t, err, storage.ErrURLNotFound)
	})

	t.Run("Not Found", func(t *testing.T) {
		_, err := s.DeleteURL("alias_that_does_not_exist")
		require.ErrorIs(t, err, storage.ErrURLNotFound)
	})
}
