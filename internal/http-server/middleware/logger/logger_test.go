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

type mockHandler struct {
	records *[]slog.Record
	attrs   []slog.Attr
}

func newMockHandler() *mockHandler {
	records := make([]slog.Record, 0)
	return &mockHandler{records: &records}
}

func (h *mockHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *mockHandler) Handle(_ context.Context, r slog.Record) error {
	if len(h.attrs) > 0 {
		r.AddAttrs(h.attrs...)
	}
	*h.records = append(*h.records, r)
	return nil
}

func (h *mockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &mockHandler{
		records: h.records,
		attrs:   append(append([]slog.Attr{}, h.attrs...), attrs...),
	}
}

func (h *mockHandler) WithGroup(name string) slog.Handler { return h }

func getAttr(rec slog.Record, key string) (slog.Value, bool) {
	var val slog.Value
	found := false

	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			val = a.Value
			found = true
			return false
		}
		return true
	})
	return val, found
}

func TestLoggerMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		reqIDHeader string
		respStatus  int
		respBody    string

		// Ожидания для логов
		wantMethod string
		wantPath   string
		wantStatus int64
		wantBytes  int64
		wantReqID  string
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
			wantReqID:   "",
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

			h := newMockHandler()
			log := slog.New(h)
			mw := New(log)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.respStatus)
				w.Write([]byte(tc.respBody))
			})

			handler := middleware.RequestID(mw(testHandler))

			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.reqIDHeader != "" {
				req.Header.Set("X-Request-ID", tc.reqIDHeader)
			}

			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, tc.respStatus, rr.Code)
			require.Equal(t, tc.respBody, rr.Body.String())
			require.NotEmpty(t, *h.records, "expected log records, got none")
			rec := (*h.records)[len(*h.records)-1]

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

			for _, key := range []string{"method", "path", "status", "bytes"} {
				if !found[key] {
					t.Errorf("log does not contain %s", key)
				}
			}

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
