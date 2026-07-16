Надо удалить лишнее (что осталось от psql)


проект сделан по видео:
https://www.youtube.com/watch?v=rCJvW2xgnk0
==============================
Запуск проекта:

Установить переменную окружения:
CONFIG_PATH=config/local.yaml

запустить:
go run cmd/url-shortener/main.go

==============================
тесты

проверить выполнение всех тестов
go test ./... -v

🧪 Проверка покрытия тестами
go test ./... -cover

================================
запуск контенера с бд
docker-compose up -d

удалить контенер вместе с volume
docker compose down -v

================================
установка миграций:

В терминале (в корне проекта) выполните:
go get -u github.com/golang-migrate/migrate/v4
go get -u github.com/golang-migrate/migrate/v4/database/postgres
go get -u github.com/golang-migrate/migrate/v4/source/file

Создаем папку и файлы миграций:
migrations/000001_create_urls_table.up.sql (Создание таблицы)
migrations/000001_create_urls_table.down.sql (Откат создания)
migrations/000002_add_seed_data.up.sql (Тестовое заполнение)
migrations/000002_add_seed_data.down.sql (Откат сида)

Добавляем запуск миграций в приложение:
Нужно создать новый файл internal/storage/postgres/migrations.go. Сделаем так, чтобы при запуске в режиме local (или dev), приложение само накатывало миграции перед стартом.

Вызываем миграции в main.go

====================================
Тестируем storage слой

Тестировать хендлеры через моки — это отлично, но слой работы с базой данных (internal/storage/postgres) — это самое критичное место. Там сидят SQL-запросы, и если в них ошибка (например, не тот тип данных или кривой RETURNING), моки этого не покажут.

Тесты для БД называются интеграционными. Идеальный подход в Go для этого сейчас — использовать Testcontainers.

Что такое Testcontainers?
Вместо того чтобы требовать от разработчика поднять локальную БД или использовать какую-то общую тестовую базу (где тесты могут мешать друг другу), Testcontainers при запуске тестов сам скачивает и запускает изолированный контейнер PostgreSQL. Когда тесты заканчиваются — он его удаляет. База всегда чистая, как слеза.

Устанавливаем Testcontainers
В терминале выполните:
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres

пояснение:
github.com/testcontainers/testcontainers-go — ядро библиотеки для управления Docker-контейнерами из кода.
github.com/testcontainers/testcontainers-go/modules/postgres — специфичный модуль для Postgres, чтобы не писать конфигурацию контейнера с нуля (он уже знает про юзера, пароль и стандартные порты).


запуск тестов storage
go test -v ./internal/storage/postgres -count=1


========================================
создаем тег для workflow:

PS C:\codemanya\actual\2_url_shortener_rest_api> git tag v0.0.1
PS C:\codemanya\actual\2_url_shortener_rest_api> git push rest-url-shortener v0.0.1
Total 0 (delta 0), reused 0 (delta 0), pack-reused 0 (from 0)
To https://github.com/Saul-Goodman-art/rest-url-shortener.git
* [new tag]         v0.0.1 -> v0.0.1
  PS C:\codemanya\actual\2_url_shortener_rest_api> 




