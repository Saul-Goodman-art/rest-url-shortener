package tests

import (
	"github.com/brianvoe/gofakeit/v6" // Библиотека, которая позволяет генерировать случайные данные: имена людей телефоны имэйлы и такое прочее
	"net/http"
	"net/url"
	"testing"

	// Он не запускает сервер — он делает реальные HTTP‑запросы.
	"github.com/gavv/httpexpect/v2" // Библиотека или framework, который нужен чтобы тестировать http сервисы
	"github.com/stretchr/testify/require"

	"url-shortener/internal/http-server/handlers/url/save"
	"url-shortener/internal/lib/api"
	"url-shortener/internal/lib/random"
)

const (
	host = "localhost:8082"
)

// простеньк тест для понимания
/*
🧠 Что именно тестирует?
	✔️ Что POST /url работает
	✔️ Что BasicAuth проходит
	✔️ Что JSON валиден
	✔️ Что сервер возвращает alias
	✔️ Что сервер запущен и доступен
*/
func TestURLShortener_HappyPath(t *testing.T) {

	// Здесь создаётся HTTP‑клиент, который будет отправлять запросы на:http://localhost:8082
	u := url.URL{
		Scheme: "http",
		Host:   host,
	}
	e := httpexpect.Default(t, u.String())

	//
	e.POST("/url"). // Отправляем POST‑запрос
		WithJSON(save.Request{ // какой будет джесон в теле запроса
			URL:   gofakeit.URL(),
			Alias: random.NewRandomString(10),
		}).
		WithBasicAuth("myuser", "mypass").
		Expect().
		Status(200).
		JSON().Object().
		ContainsKey("alias")
}

//nolint:funlen
func TestURLShortener_SaveRedirect(t *testing.T) {
	testCases := []struct {
		name  string
		url   string
		alias string
		error string
	}{
		{
			name:  "Valid URL",
			url:   gofakeit.URL(),
			alias: gofakeit.Word() + gofakeit.Word(),
		},
		{
			name:  "Invalid URL",
			url:   "invalid_url",
			alias: gofakeit.Word(),
			error: "field URL is not a valid URL",
		},
		{
			name:  "Empty Alias",
			url:   gofakeit.URL(),
			alias: "",
		},
		// TODO: add more test cases
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			u := url.URL{
				Scheme: "http",
				Host:   host,
			}

			e := httpexpect.Default(t, u.String())

			// Save

			resp := e.POST("/url").
				WithJSON(save.Request{
					URL:   tc.url,
					Alias: tc.alias,
				}).
				WithBasicAuth("myuser", "mypass").
				Expect().Status(http.StatusOK).
				JSON().Object()

			if tc.error != "" {
				resp.NotContainsKey("alias")

				resp.Value("error").String().IsEqual(tc.error)

				return
			}

			alias := tc.alias

			if tc.alias != "" {
				resp.Value("alias").String().IsEqual(tc.alias)
			} else {
				resp.Value("alias").String().NotEmpty()

				alias = resp.Value("alias").String().Raw()
			}

			// Redirect

			testRedirect(t, alias, tc.url)
		})
	}
}

func testRedirect(t *testing.T, alias string, urlToRedirect string) {
	u := url.URL{
		Scheme: "http",
		Host:   host,
		Path:   alias,
	}

	redirectedToURL, err := api.GetRedirect(u.String())
	require.NoError(t, err)

	require.Equal(t, urlToRedirect, redirectedToURL)
}

// Табличный функциональный тест для DELETE.
// Тестовая запись создаётся один раз в начале, затем выполняются кейсы:
// - успешное удаление созданного алиаса
// - попытка удалить несуществующий алиас (ожидаем "not found")
// - пустой алиас: отправляем DELETE /url/ и ожидаем 405 Method Not Allowed
func TestURLShortener_Delete(t *testing.T) {
	// Не параллелим, потому что используем общую подготовку
	u := url.URL{
		Scheme: "http",
		Host:   host,
	}
	e := httpexpect.Default(t, u.String())

	// --- Подготовка: создаём тестовую запись один раз ---
	createdAlias := random.NewRandomString(10)
	createdURL := gofakeit.URL()

	createResp := e.POST("/url"). // в createResp сохр-ся результат реальн запроса
		WithJSON(save.Request{ // То есть именно этот JSON попадет в ваш хендлер save.New(...)
			URL:   createdURL,
			Alias: createdAlias,
		}).
		WithBasicAuth("myuser", "mypass").
		Expect(). // Вот здесь запрос реально отправляется на сервер.
		Status(http.StatusOK). // Проверяет, что именно сервер ответил
		JSON().Object() // Говорит библиотеке: "Ответ должен быть JSON-объектом." После этого можно обращаться к его полям.

	createResp.Value("alias").String().IsEqual(createdAlias) // Убедимся, что alias вернулся

	// --- Табличные кейсы ---
	cases := []struct {
		name      string
		kind      string // "success", "not_found", "empty_alias"
		expectErr string // текст ошибки в теле (для not_found), пусто для success, ignored for empty_alias
	}{
		{
			name:      "Success delete existing",
			kind:      "success",
			expectErr: "",
		},
		{
			name:      "Not found (delete other alias)",
			kind:      "not_found",
			expectErr: "not found",
		},
		{
			name:      "Empty alias -> 405 Method Not Allowed",
			kind:      "empty_alias",
			expectErr: "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Функциональные кейсы — используют уже созданную запись
			switch tc.kind {
			case "empty_alias":
				// Отправляем реальный HTTP запрос DELETE /url/ и ожидаем 405
				e.DELETE("/url/").
					WithBasicAuth("myuser", "mypass").
					Expect().
					Status(http.StatusMethodNotAllowed)
				return
			}

			var deleteTarget string
			var expectedURL string

			switch tc.kind {
			case "success":
				deleteTarget = createdAlias
				expectedURL = createdURL
			case "not_found":
				deleteTarget = gofakeit.UUID()
			default:
				t.Fatalf("unknown case kind: %s", tc.kind)
			}

			// Выполняем DELETE /url/{alias}
			delResp := e.DELETE("/url/"+deleteTarget).
				WithBasicAuth("myuser", "mypass").
				Expect().
				Status(http.StatusOK).
				JSON().Object()

			// Если ожидается ошибка — проверяем поля status и error
			if tc.expectErr != "" {
				delResp.Value("status").String().IsEqual("Error")
				delResp.Value("error").String().IsEqual(tc.expectErr)
				return
			}

			// Успешное удаление — проверяем deleted-url совпадает с исходным URL
			delResp.Value("deleted-url").String().IsEqual(expectedURL)

			// После удаления GET /{alias} должен вернуть not found
			getResp := e.GET("/" + createdAlias).
				Expect().
				Status(http.StatusOK).
				JSON().Object()

			getResp.Value("error").String().IsEqual("not found")
		})
	}
}
