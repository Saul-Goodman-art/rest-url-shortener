package redirect_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"url-shortener/internal/storage"

	"url-shortener/internal/http-server/handlers/redirect"
	"url-shortener/internal/http-server/handlers/redirect/mocks"
	"url-shortener/internal/lib/api"
	"url-shortener/internal/lib/logger/handlers/slogdiscard"
)

func TestRedirectIntegration(t *testing.T) {
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
			url:   "https://www.google.com/",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			urlGetterMock := mocks.NewURLGetter(t)

			if tc.respError == "" || tc.mockError != nil {
				urlGetterMock.On("GetURL", tc.alias).
					Return(tc.url, tc.mockError).Once()
			}

			r := chi.NewRouter()
			r.Get("/{alias}", redirect.New(slogdiscard.NewDiscardLogger(), urlGetterMock))

			ts := httptest.NewServer(r)
			defer ts.Close()

			redirectedToURL, err := api.GetRedirect(ts.URL + "/" + tc.alias)
			require.NoError(t, err)

			// Check the final URL after redirection.
			assert.Equal(t, tc.url, redirectedToURL)
		})
	}
}

func TestRedirectHandler(t *testing.T) {
	cases := []struct {
		name string

		alias string

		url string

		mockError error

		expectedStatus int

		expectedLocation string

		respError string

		shouldCallMock bool
	}{
		{
			name:             "Success",
			alias:            "google",
			url:              "https://google.com",
			expectedStatus:   http.StatusFound,
			expectedLocation: "https://google.com",
			shouldCallMock:   true,
		},
		{
			name:           "Alias empty",
			expectedStatus: http.StatusBadRequest,
			respError:      "invalid request",
		},
		{
			name:           "Not found",
			alias:          "missing",
			mockError:      storage.ErrURLNotFound,
			expectedStatus: http.StatusNotFound,
			respError:      "not found",
			shouldCallMock: true,
		},
		{
			name:           "Internal error",
			alias:          "google",
			mockError:      errors.New("boom"),
			expectedStatus: http.StatusInternalServerError,
			respError:      "internal error",
			shouldCallMock: true,
		},
		{
			name:           "URL returned together with error",
			alias:          "google",
			url:            "https://google.com",
			mockError:      errors.New("boom"),
			expectedStatus: http.StatusInternalServerError,
			respError:      "internal error",
			shouldCallMock: true,
		},
	}

	for _, tc := range cases {

		t.Run(tc.name, func(t *testing.T) {

			urlGetterMock := mocks.NewURLGetter(t)

			if tc.shouldCallMock {
				urlGetterMock.
					On("GetURL", tc.alias).
					Return(tc.url, tc.mockError).
					Once()
			}

			handler := redirect.New(
				slogdiscard.NewDiscardLogger(),
				urlGetterMock,
			)

			// Router для нормального прохождения через chi (непустые alias)
			r := chi.NewRouter()
			r.Get("/{alias}", handler)

			var req *http.Request
			var rr *httptest.ResponseRecorder

			if tc.alias == "" {

				// Chi не матчит "/{alias}" для пути "/" — используем "/"
				// и вручную создаём RouteContext с пустым alias,
				// чтобы chi.URLParam внутри хендлера вернул ""

				req = httptest.NewRequest(http.MethodGet, "/", nil)

				rc := chi.NewRouteContext()
				rc.URLParams.Add("alias", "")

				req = req.WithContext(
					context.WithValue(
						req.Context(),
						chi.RouteCtxKey,
						rc,
					),
				)

				rr = httptest.NewRecorder()

				// Можно вызвать handler напрямую,
				// но важно, чтобы в контексте был RouteContext

				handler.ServeHTTP(rr, req)

			} else {

				req = httptest.NewRequest(
					http.MethodGet,
					"/"+tc.alias,
					nil,
				)

				rr = httptest.NewRecorder()

				r.ServeHTTP(rr, req)
			}

			require.Equal(t, tc.expectedStatus, rr.Code)

			if tc.respError == "" {

				require.Equal(
					t,
					tc.expectedLocation,
					rr.Header().Get("Location"),
				)

				return
			}

			require.Contains(
				t,
				rr.Header().Get("Content-Type"),
				"application/json",
			)

			require.Empty(
				t,
				rr.Header().Get("Location"),
			)

			var resp struct {
				Status string `json:"status"`
				Error  string `json:"error"`
			}

			require.NoError(
				t,
				json.Unmarshal(rr.Body.Bytes(), &resp),
			)

			require.Equal(
				t,
				"Error",
				resp.Status,
			)

			require.Equal(
				t,
				tc.respError,
				resp.Error,
			)

			if !tc.shouldCallMock {

				urlGetterMock.AssertNotCalled(
					t,
					"GetURL",
					mock.Anything,
				)
			}
		})
	}
}
