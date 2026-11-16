package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"opskeeper/backend/logging"
)

func TestWriteErrorReturnsDisplayCodeAndType(t *testing.T) {
	for errorType, displayCode := range errorMessageCodes {
		t.Run(errorType, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeError(response, httptest.NewRequest(http.MethodGet, "/", nil), http.StatusBadRequest, errorType, "invalid")
			var body errorBody
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error.Code != displayCode || body.Error.Type != errorType {
				t.Fatalf("error = %+v, want code %s and type %s", body.Error, displayCode, errorType)
			}
		})
	}
}

func TestWriteInternalErrorCorrelatesResponseAndLogs(t *testing.T) {
	var output bytes.Buffer
	logger := logging.NewRaw(&output).With("service", "test-api")
	handler := middleware.RequestID(requestLogger(logger, "/", false)(requestLoggerContext(logger)(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeInternalError(writer, request, "resource lookup", errors.New("database password=secret postgres://db-user:db-secret@db.internal/opskeeper connection refused"))
	}))))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/resources", nil))

	body := response.Body.String()
	if response.Code != http.StatusInternalServerError || !strings.Contains(body, `"code":"E0018"`) || !strings.Contains(body, `"type":"internal_error"`) || !strings.Contains(body, `"message":`) || !strings.Contains(body, `"request_id":`) || strings.Contains(body, `"code":"internal_error"`) {
		t.Fatalf("error response = %d %s", response.Code, body)
	}
	if strings.Contains(body, "secret") || strings.Contains(body, "db-secret") || !strings.Contains(body, "connection refused") {
		t.Fatalf("error response detail = %s", body)
	}
	logLine := output.String()
	if !strings.Contains(logLine, "request failed") || !strings.Contains(logLine, "http-request") || !strings.Contains(logLine, "error_code=internal_error") || strings.Contains(logLine, "secret") || strings.Contains(logLine, "db-secret") {
		t.Fatalf("correlated logs = %q", logLine)
	}
}
