package logger

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//
// ===============================
//   Мок slog.Handler
// ===============================
//
// Этот мок перехватывает все slog-записи, чтобы мы могли проверить:
// - какие атрибуты были залогированы
// - что middleware действительно логирует статус, байты, метод, путь, request_id
// - что логгер вызывается корректно
//

type mockHandler struct {
	records *[]slog.Record // сюда собираем все записи
	attrs   []slog.Attr    // накопленные атрибуты через WithAttrs()
}

// Конструктор мока — создаёт пустой список записей
func newMockHandler() *mockHandler {
	records := make([]slog.Record, 0)
	return &mockHandler{records: &records}
}

// Enabled — всегда true, чтобы логгер не фильтровал записи
func (h *mockHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

// Handle — вызывается при каждой slog-записи
// Мы добавляем накопленные атрибуты и сохраняем запись в массив
func (h *mockHandler) Handle(_ context.Context, r slog.Record) error {
	if len(h.attrs) > 0 {
		r.AddAttrs(h.attrs...)
	}
	*h.records = append(*h.records, r)
	return nil
}

// WithAttrs — добавляет атрибуты к будущим записям
func (h *mockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &mockHandler{
		records: h.records,
		attrs:   append(append([]slog.Attr{}, h.attrs...), attrs...),
	}
}

// WithGroup — не используется, но обязан быть для интерфейса
func (h *mockHandler) WithGroup(name string) slog.Handler { return h }

//
// ===============================
//   Вспомогательная функция
// ===============================
//
// getAttr — достаёт конкретный атрибут из slog.Record по ключу.
// Это нужно, чтобы удобно проверять request_id, status, bytes и т.д.
//

func getAttr(rec slog.Record, key string) (slog.Value, bool) {
	var val slog.Value
	found := false

	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			val = a.Value
			found = true
			return false // прекращаем перебор, как только нашли
		}
		return true
	})

	return val, found
}

//
// ===============================
//   Основной тест middleware
// ===============================
//
// Мы используем table-driven подход, чтобы проверить разные сценарии:
// - GET с кастомным RequestID
// - POST без RequestID (chi должен сгенерировать свой)
// - DELETE с ошибкой 500
//
// Каждый кейс проверяет:
// - корректность HTTP-ответа
// - корректность логов
// - корректность статуса и количества байт
// - корректность request_id
//

func TestLoggerMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		reqIDHeader string // если пусто — chi сам создаст RequestID

		respStatus int
		respBody   string

		// Ожидания для логов
		wantMethod string
		wantPath   string
		wantStatus int64
		wantBytes  int64
		wantReqID  string // если пусто — проверяем, что не пустой
	}{
		{
			name:        "Success with custom RequestID",
			method:      "GET",
			path:        "/test",
			reqIDHeader: "req-123",
			respStatus:  http.StatusTeapot,
			respBody:    "hello",
			wantMethod:  "GET",
			wantPath:    "/test",
			wantStatus:  418,
			wantBytes:   5,
			wantReqID:   "req-123",
		},
		{
			name:        "POST without custom RequestID (chi generates)",
			method:      "POST",
			path:        "/api/save",
			reqIDHeader: "",
			respStatus:  http.StatusOK,
			respBody:    `{"status":"ok"}`,
			wantMethod:  "POST",
			wantPath:    "/api/save",
			wantStatus:  200,
			wantBytes:   15,
			wantReqID:   "", // проверяем, что не пустой
		},
		{
			name:        "Internal Server Error",
			method:      "DELETE",
			path:        "/url/abc",
			reqIDHeader: "del-999",
			respStatus:  http.StatusInternalServerError,
			respBody:    "db error",
			wantMethod:  "DELETE",
			wantPath:    "/url/abc",
			wantStatus:  500,
			wantBytes:   8,
			wantReqID:   "del-999",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			//
			// 1. Создаём мок логгера
			// Каждый тест должен иметь свой мок, чтобы записи не смешивались
			//
			h := newMockHandler()
			log := slog.New(h)
			mw := New(log)

			//
			// 2. Создаём тестовый хендлер
			// Он возвращает статус и тело, которые мы будем проверять
			//
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.respStatus)
				w.Write([]byte(tc.respBody))
			})

			//
			// 3. Собираем цепочку middleware:
			// RequestID → наш logger → хендлер
			//
			handler := middleware.RequestID(mw(testHandler))

			//
			// 4. Формируем HTTP-запрос
			// Если передан reqIDHeader — добавляем X-Request-ID
			//
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.reqIDHeader != "" {
				req.Header.Set("X-Request-ID", tc.reqIDHeader)
			}

			rr := httptest.NewRecorder()

			//
			// 5. Выполняем запрос
			//
			handler.ServeHTTP(rr, req)

			//
			// 6. Проверяем HTTP-ответ
			// Это sanity-check: middleware не должен ломать ответ
			//
			require.Equal(t, tc.respStatus, rr.Code)
			require.Equal(t, tc.respBody, rr.Body.String())

			//
			// 7. Проверяем логи
			// Первый лог — "logger middleware enabled"
			// Последний лог — завершение запроса (он нам и нужен)
			//
			require.NotEmpty(t, *h.records, "expected log records, got none")
			rec := (*h.records)[len(*h.records)-1]

			//
			// 8. Проверяем наличие всех ключевых атрибутов
			//
			found := make(map[string]bool)

			rec.Attrs(func(a slog.Attr) bool {
				found[a.Key] = true

				switch a.Key {
				case "method":
					assert.Equal(t, tc.wantMethod, a.Value.String())
				case "path":
					assert.Equal(t, tc.wantPath, a.Value.String())
				case "status":
					assert.Equal(t, tc.wantStatus, a.Value.Int64())
				case "bytes":
					assert.Equal(t, tc.wantBytes, a.Value.Int64())
				}

				return true
			})

			//
			// 9. Проверяем, что все ключи присутствуют
			//
			for _, key := range []string{"method", "path", "status", "bytes"} {
				if !found[key] {
					t.Errorf("log does not contain %s", key)
				}
			}

			//
			// 10. Проверяем request_id
			//
			if val, ok := getAttr(rec, "request_id"); ok {
				if tc.wantReqID != "" {
					assert.Equal(t, tc.wantReqID, val.String())
				} else {
					assert.NotEmpty(t, val.String(), "expected auto-generated request_id, got empty")
				}
			} else {
				t.Error("log does not contain request_id")
			}
		})
	}
}
