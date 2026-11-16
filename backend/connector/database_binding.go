package connector

import (
	"context"
	"encoding/json"
	"opskeeper/backend/engine"
)

func databaseScopedTools(add func(string, string, json.RawMessage, func(context.Context, map[string]any) (engine.ToolResult, error)), target, binding map[string]any) func(string, string, json.RawMessage, func(context.Context, map[string]any) (engine.ToolResult, error)) {
	return func(name, description string, schema json.RawMessage, invoke func(context.Context, map[string]any) (engine.ToolResult, error)) {
		add(name, description, schema, func(ctx context.Context, args map[string]any) (engine.ToolResult, error) {
			result, err := invoke(ctx, args)
			result.Scope = map[string]any{"connection_target": target, "schema": args["schema"], "application_binding": binding, "authentication": "resource diagnostic credentials", "application_attribution": "not_filtered; tool-defined database or instance scope may include other applications"}
			return result, err
		})
	}
}
