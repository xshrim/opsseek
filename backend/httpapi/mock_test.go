package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"opsseek/backend/health"
	"opsseek/backend/identity"
)

func TestMockErrorsReturnSpecifiedFaultAndCorrelatedLogs(t *testing.T) {
	for index, fault := range mockErrors {
		number := strconv.Itoa(1001 + index)
		t.Run(number, func(t *testing.T) {
			var logs bytes.Buffer
			router := NewRouter(slog.New(slog.NewJSONHandler(&logs, nil)), health.NewService("test-api", time.Second, nil), testBuild, Options{
				BasePath: "/test", Identity: &stubIdentityService{},
			}, nil, nil)
			request := httptest.NewRequest(http.MethodGet, "/test/api/v1/mock/errors?number="+number, nil)
			request.AddCookie(&http.Cookie{Name: accessCookieName, Value: "test-session"})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			var body errorBody
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			messageCode := fmt.Sprintf("M%04d", index+1)
			if response.Code != fault.status || body.Error.Code != messageCode || body.Error.Type != "mock_"+number || body.Error.Message != fault.message {
				t.Fatalf("response = %d %+v, want %d %s", response.Code, body, fault.status, number)
			}
			if strings.Contains(response.Body.String(), `"number"`) || strings.Contains(response.Body.String(), `"message_code"`) {
				t.Fatalf("legacy error number or message_code is still exposed: %s", response.Body.String())
			}
			if body.Error.RequestID == "" || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("missing request correlation or cache protection: %+v %v", body, response.Header())
			}
			level := "WARN"
			if fault.status >= 500 {
				level = "ERROR"
			}
			for _, expected := range []string{`"msg":"request failed"`, `"level":"` + level + `"`, `"mock":true`, `"error_number":` + number, `"error_type":"mock_` + number + `"`, fault.message, body.Error.RequestID, `"kind":"http-request"`} {
				if !strings.Contains(logs.String(), expected) {
					t.Errorf("logs missing %q: %s", expected, logs.String())
				}
			}
		})
	}
}

func TestMockErrorsRandomAndInvalidNumbers(t *testing.T) {
	router := newAuthTestRouter(&stubIdentityService{}, false)
	for _, query := range []string{"", "?number=", "?number=9999", "?number=1011", "?number=abc", "?number=1001&number=1002", "?number=1001.0", "?number=%ZZ", "?number=1001;number=1002"} {
		t.Run(query, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/test/api/v1/mock/errors"+query, nil)
			request.AddCookie(&http.Cookie{Name: accessCookieName, Value: "test-session"})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			var body errorBody
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if query == "" {
				for index, fault := range mockErrors {
					if body.Error.Code == fmt.Sprintf("M%04d", index+1) && response.Code == fault.status && body.Error.Message == fault.message {
						return
					}
				}
				t.Fatalf("random response is not a defined fault: %d %+v", response.Code, body)
			}
			if response.Code != http.StatusBadRequest || body.Error.Code != errorMessageCodes["invalid_mock_number"] || body.Error.Type != "invalid_mock_number" {
				t.Fatalf("invalid number response = %d %+v", response.Code, body)
			}
		})
	}
}

func TestMockErrorsRequireAuthentication(t *testing.T) {
	response := httptest.NewRecorder()
	newAuthTestRouter(&stubIdentityService{authenticateError: identity.ErrInvalidSession}, false).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/test/api/v1/mock/errors?number=1007", nil))
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "M0007") {
		t.Fatalf("unauthenticated response = %d %s", response.Code, response.Body.String())
	}
}
