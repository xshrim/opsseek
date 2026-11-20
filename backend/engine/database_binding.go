package engine

import (
	"encoding/json"
	"fmt"
	"strings"
)

func IsDatabaseKind(kind string) bool {
	switch kind {
	case "PostgreSQL", "MySQL", "Oracle", "OceanBase", "MongoDB":
		return true
	}
	return false
}

// DatabaseConnectionTarget changes only object locators, never authentication or resource config.
func DatabaseConnectionTarget(kind string, defaults, binding map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(defaults))
	for key, value := range defaults {
		result[key] = value
	}
	if !IsDatabaseKind(kind) {
		return result, nil
	}
	if binding["target_conflict"] == true {
		return nil, fmt.Errorf("database resource has conflicting application targets; select one application before querying")
	}
	text := func(key string) string {
		value, _ := binding[key].(string)
		return strings.TrimSpace(value)
	}
	if kind != "Oracle" {
		if database := text("database"); database != "" {
			result["database"] = database
		}
	}
	if kind == "Oracle" || kind == "OceanBase" {
		service := text("service_name")
		if service == "" {
			service = text("service")
		}
		sid := text("sid")
		if kind == "Oracle" && service != "" && sid != "" {
			return nil, fmt.Errorf("application database service_name and sid are mutually exclusive")
		}
		if service != "" {
			result["service_name"] = service
			if kind == "Oracle" {
				delete(result, "sid")
			}
		} else if kind == "Oracle" && sid != "" {
			result["sid"] = sid
			delete(result, "service_name")
		}
	}
	if kind == "MongoDB" {
		if collection := text("collection"); collection != "" {
			result["collection"] = collection
		}
	}
	return result, nil
}

// DatabaseToolArguments supplies only operation fields declared by this tool.
func DatabaseToolArguments(kind string, binding map[string]any, schema []byte, args map[string]any) map[string]any {
	if !IsDatabaseKind(kind) {
		return args
	}
	var definition struct {
		Properties map[string]any `json:"properties"`
	}
	if json.Unmarshal(schema, &definition) != nil {
		return args
	}
	result := make(map[string]any, len(args)+1)
	for key, value := range args {
		result[key] = value
	}
	if _, supported := definition.Properties["schema"]; supported {
		if value, _ := result["schema"].(string); strings.TrimSpace(value) == "" {
			if schema, _ := binding["schema"].(string); strings.TrimSpace(schema) != "" {
				result["schema"] = strings.TrimSpace(schema)
			} else if kind == "MySQL" {
				if database, _ := binding["database"].(string); strings.TrimSpace(database) != "" {
					result["schema"] = strings.TrimSpace(database)
				}
			}
		}
	}
	if _, supported := definition.Properties["collection"]; supported {
		if value, _ := result["collection"].(string); strings.TrimSpace(value) == "" {
			if collection, _ := binding["collection"].(string); strings.TrimSpace(collection) != "" {
				result["collection"] = strings.TrimSpace(collection)
			}
		}
	}
	return result
}
