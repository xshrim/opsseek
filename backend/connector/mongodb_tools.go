package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opsseek/backend/engine"
	mt "opsseek/backend/tool/mongodb"
)

func (s *Service) resolveMongoDBTools(ctx context.Context, item engine.ContextResource, add func(string, string, json.RawMessage, func(context.Context, map[string]any) (engine.ToolResult, error))) error {
	if strings.EqualFold(strings.TrimSpace(item.Subtype), "agent") {
		return fmt.Errorf("MongoDB agent resources must use the MCP provider")
	}
	connection, err := s.mongoDBConnection(ctx, item)
	if err != nil {
		return err
	}
	add = databaseScopedTools(add, map[string]any{"database": connection.Database}, item.Binding)
	register := func(name, desc string, schema json.RawMessage, fn func(context.Context, mt.ConnectionInput, map[string]any) (any, error)) {
		add(name, desc, schema, func(runCtx context.Context, args map[string]any) (engine.ToolResult, error) {
			out, callErr := fn(runCtx, connection, args)
			if callErr != nil {
				return engine.ToolResult{}, mongodbError(name, callErr)
			}
			return engine.ToolResult{Output: out, Untrusted: true}, nil
		})
	}
	for _, tool := range mt.ListTools() {
		switch tool.Name {
		case "mongodb_health":
			register(tool.Name, tool.Description, mongodbDirectSchema(), func(c context.Context, i mt.ConnectionInput, _ map[string]any) (any, error) { return mt.Health(c, i) })
		case "mongodb_database_info":
			register(tool.Name, tool.Description, mongodbDirectSchema(), func(c context.Context, i mt.ConnectionInput, _ map[string]any) (any, error) {
				return mt.DatabaseInfo(c, i)
			})
		case "mongodb_collections":
			register(tool.Name, tool.Description, mongodbDirectSchema(), func(c context.Context, i mt.ConnectionInput, _ map[string]any) (any, error) {
				return mt.Collections(c, i)
			})
		case "mongodb_collection_stats":
			register(tool.Name, tool.Description, mongodbCollectionSchema(), func(c context.Context, i mt.ConnectionInput, args map[string]any) (any, error) {
				return mt.CollectionStats(c, i, mongodbCollectionInput(args))
			})
		case "mongodb_indexes":
			register(tool.Name, tool.Description, mongodbCollectionSchema(), func(c context.Context, i mt.ConnectionInput, args map[string]any) (any, error) {
				return mt.Indexes(c, i, mongodbCollectionInput(args))
			})
		case "mongodb_collection_schema":
			register(tool.Name, tool.Description, mongodbCollectionSchema(), func(c context.Context, i mt.ConnectionInput, args map[string]any) (any, error) {
				return mt.CollectionSchema(c, i, mongodbCollectionInput(args))
			})
		}
	}
	return nil
}

func mongodbDirectSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false}`)
}
func mongodbCollectionSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"collection":{"type":"string","minLength":1,"maxLength":255,"description":"Collection name; defaults to the application binding when configured."}},"additionalProperties":false}`)
}
func mongodbCollectionInput(args map[string]any) mt.CollectionInput {
	value, _ := args["collection"].(string)
	return mt.CollectionInput{Collection: value}
}

func (s *Service) mongoDBConnection(ctx context.Context, item engine.ContextResource) (mt.ConnectionInput, error) {
	in := mt.ConnectionInput{Host: stringValue(item.Config, "host"), Port: intValue(item.Config, "port"), Database: stringValue(item.Config, "database"), AuthSource: stringValue(item.Config, "auth_source"), TimeoutSeconds: intValue(item.Config, "timeout_seconds")}
	secret, configured, err := s.resourceSecret(ctx, item.ID)
	if err != nil {
		return in, err
	}
	if configured {
		var values map[string]any
		if err := json.Unmarshal(secret, &values); err != nil {
			return in, fmt.Errorf("decode MongoDB credential: %w", err)
		}
		in.Username, in.Password = stringValue(values, "username"), stringValue(values, "password")
		if in.Host == "" { in.Host = stringValue(values, "host") }
		if in.Port == 0 { in.Port = intValue(values, "port") }
		if in.Database == "" { in.Database = stringValue(values, "database") }
		if in.AuthSource == "" { in.AuthSource = stringValue(values, "auth_source") }
		if in.TimeoutSeconds == 0 { in.TimeoutSeconds = intValue(values, "timeout_seconds") }
	}
	target, err := engine.DatabaseConnectionTarget(item.Kind, map[string]any{"database": in.Database}, item.Binding)
	if err != nil {
		return in, err
	}
	if database, ok := target["database"].(string); ok && strings.TrimSpace(database) != "" {
		in.Database = strings.TrimSpace(database)
	}
	return in, nil
}
