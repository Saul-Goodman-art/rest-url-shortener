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
		name           string
		alias          string
		deletedURL     string
		mockError      error
		respError      string
		expectedURL    string
		expectedCode   int
		shouldCallMock bool
	}{
		{
			name:           "Success",
			alias:          "test_alias",
			deletedURL:     "https://google.com",
			expectedURL:    "https://google.com",
			expectedCode:   http.StatusOK,
			shouldCallMock: true,
		},
		{
			name:           "Alias empty",
			alias:          "",
			respError:      "invalid request",
			expectedCode:   http.StatusBadRequest,
			shouldCallMock: false,
		},
		{
			name:           "Not found",
			alias:          "missing",
			mockError:      storage.ErrURLNotFound,
			respError:      "not found",
			expectedCode:   http.StatusNotFound,
			shouldCallMock: true,
		},
		{
			name:           "Internal error",
			alias:          "test_alias",
			mockError:      errors.New("unexpected"),
			respError:      "internal error",
			expectedCode:   http.StatusInternalServerError,
			shouldCallMock: true,
		},

		{
			name:           "URL returned together with error",
			alias:          "test_alias",
			deletedURL:     "https://google.com",
			mockError:      errors.New("boom"),
			respError:      "internal error",
			expectedCode:   http.StatusInternalServerError,
			shouldCallMock: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			urlDeleterMock := mocks.NewURLDeleter(t)

			if tc.shouldCallMock {
				urlDeleterMock.
					On("DeleteURL", tc.alias).
					Return(tc.deletedURL, tc.mockError).
					Once()
			}

			handler := delete.New(slogdiscard.NewDiscardLogger(), urlDeleterMock)
			r := chi.NewRouter()
			r.Delete("/url/{alias}", handler)

			var req *http.Request
			var rr *httptest.ResponseRecorder

			if tc.alias == "" {
				req = httptest.NewRequest(http.MethodDelete, "/url/", nil)
				rc := chi.NewRouteContext()
				rc.URLParams.Add("alias", "")
				req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rc))

				rr = httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
			} else {
				req = httptest.NewRequest(http.MethodDelete, "/url/"+tc.alias, nil)
				rr = httptest.NewRecorder()
				r.ServeHTTP(rr, req)
			}

			require.Equal(t, tc.expectedCode, rr.Code)
			require.Contains(t,
				rr.Header().Get("Content-Type"),
				"application/json",
			)
			var resp struct {
				Status string `json:"status"`
				Error  string `json:"error"`
				URL    string `json:"deleted-url"`
			}

			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
			require.Equal(t, tc.respError, resp.Error)

			if tc.respError == "" {
				require.Equal(t, tc.expectedURL, resp.URL)
			}
			var raw map[string]any
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &raw))

			if tc.respError == "" {
				_, ok := raw["deleted-url"]
				require.True(t, ok)

				_, ok = raw["error"]
				require.False(t, ok)
			} else {
				_, ok := raw["deleted-url"]
				require.False(t, ok)

				_, ok = raw["error"]
				require.True(t, ok)
			}
			if !tc.shouldCallMock {
				urlDeleterMock.AssertNotCalled(t, "DeleteURL")
			}
			urlDeleterMock.AssertExpectations(t)
		})
	}
}
