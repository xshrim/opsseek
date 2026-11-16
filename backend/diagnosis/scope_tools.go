package diagnosis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"opsseek/backend/application"
	"opsseek/backend/authorization"
	"opsseek/backend/engine"
)

type scopeQueryReader interface {
	DiagnosticScopeTeams(context.Context, string) ([]application.DiagnosticScopeTeam, error)
	DiagnosticScopeProjects(context.Context, string, string) ([]application.DiagnosticScopeProject, error)
	DiagnosticScopeApplications(context.Context, string, string, string, string, int, int) (application.DiagnosticScopeApplicationPage, error)
	DiagnosticApplicationContextByLocator(context.Context, string, string, string) (application.DiagnosticContext, error)
}

const scopeToolSource = "diagnosis-scope"

func (s *Service) diagnosticScopeTools(authorizationContext context.Context, scopeID string) []engine.Tool {
	reader, ok := s.applications.(scopeQueryReader)
	if !ok || strings.TrimSpace(scopeID) == "" {
		return nil
	}
	return []engine.Tool{
		engine.ToolFunc{Def: engine.ToolDefinition{
			Name: "diagnosis_scope_teams", Source: scopeToolSource, ReadOnly: true,
			Description: "列出当前诊断范围内可见的团队及其项目数量。平台范围使用此工具开始了解组织结构；不要把数据库 ID 当作参数。",
			InputSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		}, Fn: func(ctx context.Context, _ map[string]any) (engine.ToolResult, error) {
			items, err := reader.DiagnosticScopeTeams(withAuthorizationContext(ctx, authorizationContext), scopeID)
			if err != nil {
				return engine.ToolResult{}, err
			}
			return engine.ToolResult{Output: map[string]any{"teams": items}, Untrusted: true}, nil
		}},
		engine.ToolFunc{Def: engine.ToolDefinition{
			Name: "diagnosis_scope_projects", Source: scopeToolSource, ReadOnly: true,
			Description: "列出当前诊断范围内可见的项目及应用数量。team_name 只能填写团队名称，不能填写项目名称；不要传内部 ID。",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"team_name":{"type":"string","maxLength":200,"description":"团队名称（不是项目名称），用于缩小项目范围；不是数据库内部 ID。"}},"additionalProperties":false}`),
		}, Fn: func(ctx context.Context, args map[string]any) (engine.ToolResult, error) {
			items, err := reader.DiagnosticScopeProjects(withAuthorizationContext(ctx, authorizationContext), scopeID, scopeToolString(args, "team_name"))
			if err != nil {
				return engine.ToolResult{}, err
			}
			return engine.ToolResult{Output: map[string]any{"projects": items}, Untrusted: true}, nil
		}},
		engine.ToolFunc{Def: engine.ToolDefinition{
			Name: "diagnosis_scope_applications", Source: scopeToolSource, ReadOnly: true,
			Description: "列出团队或项目中的应用摘要（不是单个应用详情）。用户问‘某团队/当前团队有哪些应用’时必须调用本工具并填写 team_name；问‘某项目/当前项目有哪些应用’时调用本工具并填写 project_name。team_name 只能是实际团队名称，project_name 只能是实际项目名称。不要用 diagnosis_application_context 查询团队或项目的应用清单。结果支持分页，limit 最大 100。",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"team_name":{"type":"string","maxLength":200,"description":"团队名称。列出某团队有哪些应用时填写团队名；绝不能填写项目名。"},"project_name":{"type":"string","maxLength":200,"description":"项目名称。列出某项目有哪些应用时填写项目名；绝不能填写团队名。"},"project_code":{"type":"string","maxLength":100,"description":"项目编码，可与 project_name 组合过滤；不是数据库内部 ID。"},"offset":{"type":"integer","minimum":0,"maximum":1000000,"description":"从零开始的分页偏移量；使用上一页返回的 next_offset 继续。"},"limit":{"type":"integer","minimum":1,"maximum":100,"description":"本页最多返回的应用数量，最大为 100。"}},"additionalProperties":false}`),
		}, Fn: func(ctx context.Context, args map[string]any) (engine.ToolResult, error) {
			page, err := reader.DiagnosticScopeApplications(withAuthorizationContext(ctx, authorizationContext), scopeID, scopeToolString(args, "team_name"), scopeToolString(args, "project_name"), scopeToolString(args, "project_code"), scopeToolInt(args, "offset", 0), scopeToolInt(args, "limit", 100))
			if err != nil {
				return engine.ToolResult{}, err
			}
			return engine.ToolResult{Output: page, Untrusted: true}, nil
		}},
		engine.ToolFunc{Def: engine.ToolDefinition{
			Name: "diagnosis_application_context", Source: scopeToolSource, ReadOnly: true,
			Description: "仅用于读取一个已知具体应用的配置和拓扑，且 application_context 尚未包含该应用时才调用。绝不能用于列出团队/项目中的应用、统计应用或查找应用清单；这些问题必须调用 diagnosis_scope_applications。参数只能是一个明确的应用名称或编码，绝不能把团队名、项目名或 selected_resources 中的资源名称作为应用名称查询。未找到应用只表示该应用候选不存在；应继续按用户问题使用其他匹配工具。",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"application_name":{"type":"string","maxLength":200,"description":"应用名称；仅用于查应用，不能填资源名称"},"application_code":{"type":"string","maxLength":100,"description":"应用编码；仅用于查应用"}},"anyOf":[{"required":["application_name"]},{"required":["application_code"]}],"additionalProperties":false}`),
		}, Fn: func(ctx context.Context, args map[string]any) (engine.ToolResult, error) {
			name, code := scopeToolString(args, "application_name"), scopeToolString(args, "application_code")
			if name == "" && code == "" {
				return engine.ToolResult{}, fmt.Errorf("application_name or application_code is required")
			}
			item, err := reader.DiagnosticApplicationContextByLocator(withAuthorizationContext(ctx, authorizationContext), scopeID, name, code)
			if err != nil {
				if errors.Is(err, application.ErrNotFound) {
					return engine.ToolResult{Output: map[string]any{
						"found":       false,
						"object_type": "application",
						"lookup":      map[string]string{"application_name": name, "application_code": code},
						"next_step":   "This only rules out this application candidate. Check the user's selected_resources and continue with the matching resource-specific tools; do not conclude that a resource with the same name is missing.",
					}, Untrusted: true}, nil
				}
				return engine.ToolResult{}, err
			}
			raw, err := json.Marshal(item)
			if err != nil {
				return engine.ToolResult{}, err
			}
			sanitized, err := sanitizeModelJSON(raw)
			if err != nil {
				return engine.ToolResult{}, err
			}
			return engine.ToolResult{Output: sanitized, Untrusted: true}, nil
		}},
	}
}

// The model SDK owns the tool context and is not required to preserve HTTP
// context values. Reattach only server-resolved authorization filters while
// retaining the SDK context's cancellation and deadline.
func withAuthorizationContext(ctx, source context.Context) context.Context {
	if filter, ok := authorization.ResourceFilterFromContext(source); ok {
		ctx = authorization.WithResourceFilter(ctx, filter)
	}
	if filter, ok := authorization.ResourceUseFilterFromContext(source); ok {
		ctx = authorization.WithResourceUseFilter(ctx, filter)
	}
	if filter, ok := authorization.ResourceReadFilterFromContext(source); ok {
		ctx = authorization.WithResourceReadFilter(ctx, filter)
	}
	if filter, ok := authorization.ScopeFilterFromContext(source); ok {
		ctx = authorization.WithScopeFilter(ctx, filter)
	}
	return ctx
}

func scopeToolString(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func scopeToolInt(args map[string]any, key string, fallback int) int {
	switch value := args[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return fallback
	}
}
