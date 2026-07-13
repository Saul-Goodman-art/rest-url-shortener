package tests

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"log/slog"

	"url-shortener/internal/http-server/handlers/redirect"
	"url-shortener/internal/http-server/handlers/url/delete"
	"url-shortener/internal/http-server/handlers/url/save"
	"url-shortener/internal/storage/sqlite"
)

/*
Этот тест проверяет корректность HTTP‑маршрутизации всего сервера, включая:

		работу middleware,
		работу BasicAuth,
		работу хендлеров,
		работу SQLite‑storage,
		работу redirect,
		корректность путей,
		корректность статусов,
		корректность поведения клиента.
*/

func basicAuth(user, pass string) string {
	token := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	return "Basic " + token
}

func TestRouting_TableDriven(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// in-memory sqlite
	st, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("failed to init sqlite: %v", err)
	}

	// build router
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// создает новый роутер для теста, из мейн юсать нельзя
	router.Route("/url", func(r chi.Router) {
		users := map[string]string{
			"myuser": "mypass",
		}
		r.Use(middleware.BasicAuth("url-shortener", users))

		r.Post("/", save.New(log, st))
		r.Delete("/{alias}", delete.New(log, st))
	})

	router.Get("/{alias}", redirect.New(log, st))

	srv := httptest.NewServer(router)
	defer srv.Close()

	// client that does NOT follow redirects
	clientNoRedirect := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Step 1: save URL to get alias
	var savedAlias string
	{
		//Это имитация реального запроса:
		req, _ := http.NewRequest("POST", srv.URL+"/url", strings.NewReader(`{"url":"https://go.dev"}`))
		req.Header.Set("Authorization", basicAuth("myuser", "mypass"))
		req.Header.Set("Content-Type", "application/json")

		//Это реальный HTTP‑запрос к поднятому тестовому серверу
		resp, _ := http.DefaultClient.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 from save, got %d", resp.StatusCode)
		}

		//Читаем тело ответа
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Parsiм Джейсон, достаем только alias
		var data struct {
			Alias string `json:"alias"`
		}
		json.Unmarshal(body, &data)

		savedAlias = data.Alias // содержит реальный алиас из запроса
	}

	// Table-driven routing tests
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		auth       bool
		expectCode int
		client     *http.Client
	}{
		{
			name:       "POST /url without auth → 401",
			method:     "POST",
			path:       "/url",
			body:       `{"url":"https://go.dev"}`,
			auth:       false,
			expectCode: http.StatusUnauthorized,
			client:     http.DefaultClient,
		},
		{
			name:       "GET /{alias} → 302 redirect",
			method:     "GET",
			path:       "/" + savedAlias,
			body:       "",
			auth:       false,
			expectCode: http.StatusFound,
			client:     clientNoRedirect,
		},
		{
			name:       "DELETE /url/{alias} with auth → 200",
			method:     "DELETE",
			path:       "/url/" + savedAlias,
			body:       "",
			auth:       true,
			expectCode: http.StatusOK,
			client:     http.DefaultClient,
		},
	}
	// Случай Сохранения в БД проверено ранее

	// То что было до цикла эта подготовка данных необходимых для теста . Это я про запрос пост который был сделан ранее .

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, srv.URL+tc.path, strings.NewReader(tc.body))

			if tc.auth {
				req.Header.Set("Authorization", basicAuth("myuser", "mypass"))
			}
			if tc.method == "POST" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, _ := tc.client.Do(req)

			if resp.StatusCode != tc.expectCode {
				t.Errorf("expected %d, got %d", tc.expectCode, resp.StatusCode)
			}
		})
	}
}
