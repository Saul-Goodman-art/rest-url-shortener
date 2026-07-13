package save_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"url-shortener/internal/http-server/handlers/url/save"
	"url-shortener/internal/http-server/handlers/url/save/mocks"
	"url-shortener/internal/lib/logger/handlers/slogdiscard"
)

// Мог генерируется командой из файла save.go (//go:generate go run ...
func TestSaveHandler(t *testing.T) {
	cases := []struct {
		name      string
		alias     string
		url       string
		respError string
		mockError error
	}{
		{
			name:  "Success",
			alias: "test_alias",
			url:   "https://google.com",
		},
		{
			name:  "Empty alias",
			alias: "",
			url:   "https://google.com",
		},
		{
			name:      "Empty URL",
			url:       "",
			alias:     "some_alias",
			respError: "field URL is a required field",
		},
		{
			name:      "Invalid URL",
			url:       "some invalid URL",
			alias:     "some_alias",
			respError: "field URL is not a valid URL",
		},
		{
			name:      "SaveURL Error",
			alias:     "test_alias",
			url:       "https://google.com",
			respError: "failed to add url",
			mockError: errors.New("unexpected error"),
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel() // Этот метод разрешает параллельное выполнение тестов в одном пакете. Тесты, которые помечены t.Parallel(), будут выполняться одновременно (в разных горутинах), а не последовательно друг за другом.

			// мок для имитации хранилища
			urlSaverMock := mocks.NewURLSaver(t)

			if tc.respError == "" || tc.mockError != nil { // Ожидается успешный ответ или мог должен вернуть ошибку

				//Это настройка поведения мока — мы говорим моку: "Когда вызовут метод SaveURL с определёнными аргументами, верни такие-то значения".
				urlSaverMock.On("SaveURL", tc.url, mock.AnythingOfType("string")). // мок ждет метод SaveURL и любую строку
													Return(int64(1), tc.mockError). // вренте айдишник равный 1 (int64(1)) и ошибку, котор посмотрит в тесткейсе
													Once()                          // Этот мок должен быть вызван один раз
			}

			// slogdiscard.NewDiscardLogger() - Это логика заглушка, с помощью него игнорируются все логи
			handler := save.New(slogdiscard.NewDiscardLogger(), urlSaverMock)

			// создаёт JSON-строку для тела HTTP-запроса, подставляя значения url и alias из тестового случая.
			input := fmt.Sprintf(`{"url": "%s", "alias": "%s"}`, tc.url, tc.alias)

			// Кстати здесь (http.NewRequest) под капотом в запрос добавляется контекст бэкграунд
			req, err := http.NewRequest(http.MethodPost, "/save", bytes.NewReader([]byte(input))) // "/save" ??
			require.NoError(t, err)                                                               // Это проверка, что ошибка равна nil (то есть операция выполнилась успешно). Если ошибка не nil — тест немедленно останавливается и падает.

			// можо было также и так:
			//assert.NoError(t, err) // Assert добавит информацию о том что что-то пошло не так и тест пойдет дальше, а вот require.NoError(t, err) прекратит выполнение теста прямо на этом же месте .
			// Assert может быть полезен если мы хотим проанализировать проверить несколько параметров . Нам нет смысла прекращать выполнение теста на каждом плохом параметре

			//
			rr := httptest.NewRecorder() // Это создание записывающего ответа (ResponseRecorder) — специального объекта для тестирования HTTP-обработчиков. Он имитирует http.ResponseWriter, но вместо отправки ответа клиенту запоминает его в памяти.

			/*
				Эта строка вызывает ваш HTTP-хендлер напрямую, передавая ему:
					rr (httptest.ResponseRecorder) — для записи ответа
					req (http.Request) — запрос, который нужно обработать
			*/
			handler.ServeHTTP(rr, req)

			expectedStatus := http.StatusOK

			if tc.respError != "" && tc.mockError == nil {
				expectedStatus = http.StatusBadRequest
			}

			if tc.mockError != nil {
				expectedStatus = http.StatusInternalServerError
			}

			require.Equal(t, expectedStatus, rr.Code)

			body := rr.Body.String() // извлекает тело ответа из httptest.ResponseRecorder в виде строки, чтобы потом проанализировать его содержимое.

			//
			var resp save.Response

			require.NoError(t, json.Unmarshal([]byte(body), &resp)) //преобразует (десериализует) JSON-ответ от сервера в структуру Response, и проверяет, что преобразование прошло без ошибок (Здесь нашем проекте такие ошибки маловероятны, но все таки нужно сделать на всякий ).

			/*
				tc.respError  // Ожидаемая ошибка из тестового кейса
				resp.Error    // Фактическая ошибка из ответа хендлера
			*/
			require.Equal(t, tc.respError, resp.Error)

			// TODO: add more checks
		})
	}
}

func TestSaveHandler_InvalidJSON(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		wantStatus int
		wantError  string
	}{
		{
			name:       "Malformed JSON",
			input:      `{"url": "https://google.com", "alias":`, // обрыв JSON
			wantStatus: http.StatusBadRequest,
			wantError:  "failed to decode request",
		},
		{
			name:       "Not JSON at all",
			input:      `not json at all`,
			wantStatus: http.StatusBadRequest,
			wantError:  "failed to decode request",
		},
		{
			name:       "Valid JSON but missing fields",
			input:      `{"wrong": "field"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "field URL is a required field",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// мок хранилища — не должен вызываться при невалидном JSON
			urlSaverMock := mocks.NewURLSaver(t)

			handler := save.New(slogdiscard.NewDiscardLogger(), urlSaverMock)

			req := httptest.NewRequest(http.MethodPost, "/save", bytes.NewReader([]byte(tc.input)))
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			// Проверяем статус
			require.Equal(t, tc.wantStatus, rr.Code)

			// Парсим JSON-ответ
			var resp save.Response
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

			// Проверяем текст ошибки
			require.Equal(t, tc.wantError, resp.Error)

			// Проверяем, что мок НЕ вызывался
			urlSaverMock.AssertNotCalled(t, "SaveURL", mock.Anything, mock.Anything)
		})
	}
}
