package connector

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"opsseek/backend/engine"
)

func TestHostToolDescriptionsExplainEvidenceScope(t *testing.T) {
	service := &Service{}
	definitions := map[string]engine.ToolDefinition{}
	add := func(name, description string, _ json.RawMessage, _ func(context.Context, map[string]any) (engine.ToolResult, error)) {
		definitions[name] = engine.ToolDefinition{Name: name, Description: description}
	}
	if err := service.resolveHostTools(context.Background(), engine.ContextResource{ID: "host-1"}, add); err != nil {
		t.Fatal(err)
	}
	for name, terms := range map[string][]string{
		"host_metrics":   {"host-wide", "not an individual application process"},
		"host_processes": {"process identity", "start time", "CPU and memory", "process-level"},
		"host_file_logs": {"regular Linux host file path", "does not capture", "stdout"},
	} {
		definition, ok := definitions[name]
		if !ok {
			t.Fatalf("missing tool definition %q", name)
		}
		for _, term := range terms {
			if !strings.Contains(definition.Description, term) {
				t.Errorf("%s description missing %q: %s", name, term, definition.Description)
			}
		}
	}
}
