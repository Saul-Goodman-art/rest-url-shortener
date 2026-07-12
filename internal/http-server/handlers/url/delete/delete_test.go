package delete_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"url-shortener/internal/http-server/handlers/url/delete"
	"url-shortener/internal/http-server/handlers/url/delete/mocks"
	"url-shortener/internal/lib/logger/handlers/slogdiscard"
	"url-shortener/internal/storage"
)

func TestDeleteHandler(t *testing.T) {
	cases := []struct {
		name       string
		alias      string
		deletedURL string
		mockError  error
		respError  string
	}{
		{
			name:       "Success",
			alias:      "test_alias",
			deletedURL: "https://google.com",
		},
		{
			name:      "Alias empty",
			alias:     "",
			respError: "invalid request",
		},
		{
			name:      "Not found",
			alias:     "missing",
			mockError: storage.ErrURLNotFound,
			respError: "not found",
		},
		{
			name:      "Internal error",
			alias:     "test_alias",
			mockError: errors.New("unexpected"),
			respError: "internal error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			urlDeleterMock := mocks.NewURLDeleter(t)

			if tc.alias != "" {
				urlDeleterMock.
					On("DeleteURL", tc.alias).
					Return(tc.deletedURL, tc.mockError).
					Once()
			}

			handler := delete.New(slogdiscard.NewDiscardLogger(), urlDeleterMock)

			// Router для нормального прохождения через chi (непустые alias)
			r := chi.NewRouter()
			r.Delete("/url/{alias}", handler)

			var req *http.Request
			var rr *httptest.ResponseRecorder

			if tc.alias == "" {
				// Chi не матчит "/url/{alias}" для пути "/url" — используем "/url/" и вручную
				// создаём RouteContext с пустым alias, чтобы chi.URLParam внутри хендлера вернул ""
				req = httptest.NewRequest(http.MethodDelete, "/url/", nil)
				rc := chi.NewRouteContext()
				rc.URLParams.Add("alias", "")
				req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

				rr = httptest.NewRecorder()
				// Можно вызвать handler напрямую, но важно, чтобы в контексте был RouteContext
				handler.ServeHTTP(rr, req)
			} else {
				// Нормальный путь через роутер
				req = httptest.NewRequest(http.MethodDelete, "/url/"+tc.alias, nil)
				rr = httptest.NewRecorder()
				r.ServeHTTP(rr, req)
			}

			var resp struct {
				Status string `json:"status"`
				Error  string `json:"error"`
				URL    string `json:"deleted-url"`
			}

			// Убедимся, что тело — валидный JSON от нашего хендлера
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
			require.Equal(t, tc.respError, resp.Error)

			if tc.respError == "" {
				require.Equal(t, tc.deletedURL, resp.URL)
			}
		})
	}
}
