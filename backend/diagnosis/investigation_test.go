package diagnosis

import (
	"encoding/json"
	"errors"
	"testing"

	"opsseek/backend/engine"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func TestInvestigationFailureReasonsExplainsToolBudget(t *testing.T) {
	reasons := investigationFailureReasons(engine.Result{ErrorCode: "tool_budget", ErrorMessage: "tool call budget exceeded"}, errors.New("tool call budget exceeded"))
	if len(reasons) != 1 || reasons[0] != "调查范围较大，单轮工具调用预算已用尽；此前收集的证据已保留，主智能体可据此继续分析或缩小范围后补充调查。" {
		t.Fatalf("tool budget reasons = %#v", reasons)
	}
}

func TestInvestigationOutputSchemaAcceptsStructuredEntries(t *testing.T) {
	var schemaValue any
	if err := json.Unmarshal([]byte(investigationOutputSchema), &schemaValue); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("memory://investigation-schema.json", schemaValue); err != nil {
		t.Fatalf("add schema: %v", err)
	}
	schema, err := compiler.Compile("memory://investigation-schema.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	var report any
	if err := json.Unmarshal([]byte(`{
		"facts":[{"path":"backend/main.go","summary":"服务入口"}],
		"inferences":[],
		"unchecked":[{"scope":"未检查依赖服务"}],
		"suggestions":[{"next":"检查运行日志"}]
	}`), &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if err := schema.Validate(report); err != nil {
		t.Fatalf("schema rejected structured investigation report: %v", err)
	}
}

func TestDecodeInvestigationResultAcceptsStructuredEntries(t *testing.T) {
	result, err := decodeInvestigationResult(`{
		"facts":[{"path":"backend/main.go","summary":"服务入口"}],
		"inferences":["入口可用"],
		"unchecked":[{"scope":"未检查依赖服务"}],
		"suggestions":[{"next":"检查运行日志"}]
	}`)
	if err != nil {
		t.Fatalf("decodeInvestigationResult() error = %v", err)
	}
	if len(result.Facts) != 1 || result.Facts[0] != `{"path":"backend/main.go","summary":"服务入口"}` {
		t.Fatalf("facts = %#v", result.Facts)
	}
	if len(result.Inferences) != 1 || result.Inferences[0] != "入口可用" {
		t.Fatalf("inferences = %#v", result.Inferences)
	}
	if len(result.Unchecked) != 1 || result.Unchecked[0] != `{"scope":"未检查依赖服务"}` {
		t.Fatalf("unchecked = %#v", result.Unchecked)
	}
	if len(result.Suggestions) != 1 || result.Suggestions[0] != `{"next":"检查运行日志"}` {
		t.Fatalf("suggestions = %#v", result.Suggestions)
	}
}

func TestInvestigationTargetKeepsServerContextOutOfModelPayload(t *testing.T) {
	target := investigationTarget{
		Ref:     "target_1",
		Name:    "orders",
		Kind:    "PostgreSQL",
		context: json.RawMessage(`{"applications":[{"internal":"large server-only context"}]}`),
	}
	encoded, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("marshal target: %v", err)
	}
	if string(encoded) != `{"ref":"target_1","name":"orders","kind":"PostgreSQL"}` {
		t.Fatalf("model target payload = %s", encoded)
	}
}
