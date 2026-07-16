package tests

import (
	"github.com/brianvoe/gofakeit/v6"
	"net/http"
	"net/url"
	"testing"

	"github.com/gavv/httpexpect/v2"
	"github.com/stretchr/testify/require"

	"url-shortener/internal/http-server/handlers/url/save"
	"url-shortener/internal/lib/api"
	"url-shortener/internal/lib/random"
)

const (
	host = "localhost:8082"
)

func TestURLShortener_HappyPath(t *testing.T) {

	u := url.URL{
		Scheme: "http",
		Host:   host,
	}
	e := httpexpect.Default(t, u.String())

	e.POST("/url").
		WithJSON(save.Request{
			URL:   gofakeit.URL(),
			Alias: random.NewRandomString(10),
		}).
		WithBasicAuth("myuser", "mypass").
		Expect().
		Status(200).
		JSON().Object().
		ContainsKey("alias")
}

func TestURLShortener_SaveRedirect(t *testing.T) {
	testCases := []struct {
		name  string
		url   string
		alias string
		error string
	}{
		{
			name:  "Valid URL",
			url:   "https://go.dev",
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
			url:   "https://google.com",
			alias: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			u := url.URL{
				Scheme: "http",
				Host:   host,
			}

			e := httpexpect.Default(t, u.String())

			expectedStatus := http.StatusOK
			if tc.error != "" {
				expectedStatus = http.StatusBadRequest
			}

			resp := e.POST("/url").
				WithJSON(save.Request{
					URL:   tc.url,
					Alias: tc.alias,
				}).
				WithBasicAuth("myuser", "mypass").
				Expect().Status(expectedStatus).
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

func TestURLShortener_Delete(t *testing.T) {

	u := url.URL{
		Scheme: "http",
		Host:   host,
	}
	e := httpexpect.Default(t, u.String())

	createdAlias := random.NewRandomString(10)
	createdURL := gofakeit.URL()

	createResp := e.POST("/url").
		WithJSON(save.Request{
			URL:   createdURL,
			Alias: createdAlias,
		}).
		WithBasicAuth("myuser", "mypass").
		Expect().
		Status(http.StatusOK).
		JSON().Object()

	createResp.Value("alias").String().IsEqual(createdAlias)
	cases := []struct {
		name           string
		kind           string
		expectErr      string
		expectedStatus int
	}{
		{
			name:           "Success delete existing",
			kind:           "success",
			expectErr:      "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Not found (delete other alias)",
			kind:           "not_found",
			expectErr:      "not found",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Empty alias -> 405 Method Not Allowed",
			kind:           "empty_alias",
			expectErr:      "",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var deleteTarget string
			var expectedURL string

			switch tc.kind {
			case "empty_alias":
				e.DELETE("/url/").
					WithBasicAuth("myuser", "mypass").
					Expect().
					Status(tc.expectedStatus)
				return
			case "success":
				deleteTarget = createdAlias
				expectedURL = createdURL
			case "not_found":
				deleteTarget = gofakeit.UUID()
			default:
				t.Fatalf("unknown case kind: %s", tc.kind)
			}

			delResp := e.DELETE("/url/"+deleteTarget).
				WithBasicAuth("myuser", "mypass").
				Expect().
				Status(tc.expectedStatus).
				JSON().Object()
			if tc.expectErr != "" {
				delResp.Value("status").String().IsEqual("Error")
				delResp.Value("error").String().IsEqual(tc.expectErr)
				return
			}

			delResp.Value("deleted-url").String().IsEqual(expectedURL)
			getResp := e.GET("/" + createdAlias).
				Expect().
				Status(http.StatusNotFound).
				JSON().Object()

			getResp.Value("status").String().IsEqual("Error")
			getResp.Value("error").String().IsEqual("not found")
		})
	}
}

func TestURLShortener_Authorization(t *testing.T) {
	u := url.URL{
		Scheme: "http",
		Host:   host,
	}
	e := httpexpect.Default(t, u.String())
	deleteAlias := gofakeit.Word() + "_delete_auth"

	e.POST("/url").
		WithBasicAuth("myuser", "mypass").
		WithJSON(save.Request{
			URL:   gofakeit.URL(),
			Alias: deleteAlias,
		}).
		Expect().
		Status(http.StatusOK)

	tests := []struct {
		name           string
		username       string
		password       string
		method         string
		path           string
		body           interface{}
		expectedStatus int
	}{
		{
			name:           "POST /url - No auth",
			username:       "",
			password:       "",
			method:         http.MethodPost,
			path:           "/url",
			body:           save.Request{URL: "https://google.com", Alias: "test"},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "POST /url - Wrong password",
			username:       "myuser",
			password:       "wrongpass",
			method:         http.MethodPost,
			path:           "/url",
			body:           save.Request{URL: "https://google.com", Alias: "test"},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "POST /url - Wrong user",
			username:       "wronguser",
			password:       "mypass",
			method:         http.MethodPost,
			path:           "/url",
			body:           save.Request{URL: "https://google.com", Alias: "test"},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "POST /url - Valid auth",
			username:       "myuser",
			password:       "mypass",
			method:         http.MethodPost,
			path:           "/url",
			body:           save.Request{URL: gofakeit.URL(), Alias: gofakeit.Word() + "_auth_test"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "DELETE /url/alias - No auth",
			username:       "",
			password:       "",
			method:         http.MethodDelete,
			path:           "/url/test_alias",
			body:           nil,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "DELETE /url/alias - Valid auth",
			username:       "myuser",
			password:       "mypass",
			method:         http.MethodDelete,
			path:           "/url/" + deleteAlias,
			body:           nil,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := e.Request(tc.method, tc.path)

			if tc.username != "" && tc.password != "" {
				req.WithBasicAuth(tc.username, tc.password)
			}

			if tc.body != nil {
				req.WithJSON(tc.body)
			}
			req.Expect().Status(tc.expectedStatus)
		})
	}
}
