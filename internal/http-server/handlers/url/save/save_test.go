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
			t.Parallel()
			urlSaverMock := mocks.NewURLSaver(t)
			if tc.respError == "" || tc.mockError != nil {
				urlSaverMock.On("SaveURL", tc.url, mock.AnythingOfType("string")).
					Return(int64(1), tc.mockError).
					Once()
			}
			handler := save.New(slogdiscard.NewDiscardLogger(), urlSaverMock)
			input := fmt.Sprintf(`{"url": "%s", "alias": "%s"}`, tc.url, tc.alias)

			req, err := http.NewRequest(http.MethodPost, "/save", bytes.NewReader([]byte(input)))
			require.NoError(t, err)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			expectedStatus := http.StatusOK

			if tc.respError != "" && tc.mockError == nil {
				expectedStatus = http.StatusBadRequest
			}

			if tc.mockError != nil {
				expectedStatus = http.StatusInternalServerError
			}

			require.Equal(t, expectedStatus, rr.Code)
			body := rr.Body.String()
			var resp save.Response
			require.NoError(t, json.Unmarshal([]byte(body), &resp))
			require.Equal(t, tc.respError, resp.Error)
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
			input:      `{"url": "https://google.com", "alias":`,
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
			urlSaverMock := mocks.NewURLSaver(t)
			handler := save.New(slogdiscard.NewDiscardLogger(), urlSaverMock)
			req := httptest.NewRequest(http.MethodPost, "/save", bytes.NewReader([]byte(tc.input)))
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			require.Equal(t, tc.wantStatus, rr.Code)

			var resp save.Response
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

			require.Equal(t, tc.wantError, resp.Error)
			urlSaverMock.AssertNotCalled(t, "SaveURL", mock.Anything, mock.Anything)
		})
	}
}
