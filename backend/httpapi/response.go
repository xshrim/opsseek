package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code      string `json:"code"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

var errorMessageCodes = map[string]string{
	"ai_connection_error":        "E0001",
	"ai_runtime_error":           "E0002",
	"authorization_unavailable":  "E0003",
	"avatar_not_found":           "E0004",
	"avatar_too_large":           "E0005",
	"bootstrap_complete":         "E0006",
	"bootstrap_unavailable":      "E0007",
	"bundle_required":            "E0008",
	"bundle_upload_failed":       "E0009",
	"cancel_unavailable":         "E0010",
	"collaboration_unavailable":  "E0011",
	"conflict":                   "E0012",
	"connection_check_not_found": "E0013",
	"csrf_forbidden":             "E0014",
	"delete_blocked":             "E0015",
	"delete_unavailable":         "E0016",
	"forbidden":                  "E0017",
	"internal_error":             "E0018",
	"invalid_avatar":             "E0019",
	"invalid_credentials":        "E0020",
	"invalid_json":               "E0021",
	"invalid_mock_number":        "E0022",
	"invalid_repository_preview": "E0023",
	"invalid_request":            "E0024",
	"invalid_session":            "E0025",
	"invalid_upload":             "E0026",
	"mcp_unavailable":            "E0027",
	"method_not_allowed":         "E0028",
	"not_configured":             "E0029",
	"not_found":                  "E0030",
	"origin_forbidden":           "E0031",
	"parent_inactive":            "E0032",
	"password_change_required":   "E0033",
	"preferences_unavailable":    "E0034",
	"profile_unavailable":        "E0035",
	"rate_limited":               "E0036",
	"relation_cycle":             "E0037",
	"repository_preview_failed":  "E0038",
	"request_too_large":          "E0039",
	"resource_in_use":            "E0040",
	"sharing_unavailable":        "E0041",
	"target_remove_unavailable":  "E0042",
	"timeout":                    "E0043",
	"unsupported":                "E0044",
	"unknown_error":              "E0045",
}

type requestLoggerContextKey struct{}

var sensitiveErrorPattern = regexp.MustCompile(`(?i)(?:postgres(?:ql)?|mysql|redis|rediss|amqp|kafka)://[^\s,;\)\]}]+|https?://[^/@\s]+:[^/@\s]+@[^\s,;\)\]}]+|(authorization|api[- ]?key|token|secret|password|bearer|dsn|connection string)(\s*(?:[:=]|\s)\s*)[^\s,;\)\]}]+`)

func withRequestLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, requestLoggerContextKey{}, logger)
}

func requestLoggerFromContext(ctx context.Context) *slog.Logger {
	logger, _ := ctx.Value(requestLoggerContextKey{}).(*slog.Logger)
	return logger
}

func writeJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}

func writeError(writer http.ResponseWriter, request *http.Request, status int, code, message string) {
	writeLoggedError(writer, request, status, code, message)
}

func writeLoggedError(writer http.ResponseWriter, request *http.Request, status int, code, message string) {
	writeLoggedErrorNumber(writer, request, status, code, 0, message)
}

func writeLoggedErrorNumber(writer http.ResponseWriter, request *http.Request, status int, code string, number int, message string) {
	requestID := middleware.GetReqID(request.Context())
	message = publicErrorMessage(message)
	if recorder, ok := writer.(*statusRecorder); ok {
		recorder.errorCode = code
		recorder.errorReason = message
	}
	if logger := requestLoggerFromContext(request.Context()); logger != nil {
		level := slog.LevelWarn
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		attrs := []any{"kind", "error", "reqid", requestID, "status", status, "error_type", code}
		attrs = append(attrs, "error_summary", message)
		logger.Log(request.Context(), level, "request failed", attrs...)
	}
	messageCode := errorMessageCodes[code]
	if number >= 1001 && number <= 1010 && strings.HasPrefix(code, "mock_") {
		messageCode = fmt.Sprintf("M%04d", number-1000)
	}
	if messageCode == "" {
		messageCode = errorMessageCodes["unknown_error"]
	}
	writeJSON(writer, status, errorBody{Error: errorDetail{
		Code:      messageCode,
		Type:      code,
		Message:   message,
		RequestID: requestID,
	}})
}

// writeInternalError keeps the response useful while applying the same
// redaction and request correlation rules as every other API error.
func writeInternalError(writer http.ResponseWriter, request *http.Request, operation string, err error) {
	reason := publicErrorMessage(errorSummary(err))
	if reason == "" {
		reason = "unknown cause"
	}
	message := fmt.Sprintf("%s failed: %s", operation, reason)
	writeLoggedError(writer, request, http.StatusInternalServerError, "internal_error", message)
}

func errorSummary(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func publicErrorMessage(message string) string {
	message = strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	message = sensitiveErrorPattern.ReplaceAllString(message, "<redacted>")
	if len([]rune(message)) > 600 {
		message = string([]rune(message)[:600]) + "..."
	}
	return message
}
