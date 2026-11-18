package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/v2/model"
)

// This file freezes the execution contract as table-driven tests (I008 A2-2,
// R001 FR-002). The tables below are the authoritative serialization shape,
// event catalog, tool lifecycle, budget matrix, cancellation matrix and
// capability matrix. Later iterations may add columns or rows; they may not
// silently remove or rename existing entries. Request serialization uses a
// field-presence whitelist so additive fields never break the baseline.

func TestContractRequestSerializationWhitelist(t *testing.T) {
	// The whitelist is a presence assertion: every listed key must exist in
	// the serialized Request. Extra keys are allowed by design so later
	// iterations (for example I009's ExpertGroupID) can extend the contract
	// without rewriting the baseline.
	requestFieldWhitelist := []string{
		"execution_id", "actor_id", "scope_id", "purpose", "provider_id",
		"model_name", "profile", "task", "messages", "input", "context",
		"skill_id", "skill_version_id", "persona_id", "workflow_id",
		"instruction", "input_schema", "output_schema", "allowed_tools",
		"restrict_tools", "requirements", "budget", "stream",
	}
	// Fields that must never be serialized: callbacks, resolved handles and
	// execution-scoped plumbing are not part of the durable contract.
	requestFieldBlocklist := []string{
		"EventSink", "ResolvedContext", "ToolGateway", "LocatorHints",
		"ResolvedPersona", "InternalTools", "ObservationSink",
	}
	encoded, err := json.Marshal(Request{
		ExecutionID: "exec-1", ActorID: "actor-1", ScopeID: "scope-1",
		Purpose: PurposeDiagnosis, ProviderID: "provider-1", ModelName: "model-a",
		Profile: ProfileDiagnosis, Task: "inspect", Messages: []Message{{Role: "user", Content: "hi"}},
		Input:   map[string]any{"key": "value"},
		SkillID: "skill-1", SkillVersionID: "v1", PersonaID: "persona-1", WorkflowID: "wf-1",
		Instruction: "instruction", InputSchema: json.RawMessage(`{}`), OutputSchema: json.RawMessage(`{}`),
		AllowedTools: []string{"tool-a"}, RestrictTools: true,
		Requirements: Requirements{Capabilities: []string{"tool_calling"}, Streaming: true, StructuredOut: true},
		Budget:       Budget{MaxIterations: 3}, Stream: true,
		EventSink: func(Event) error { return nil }, ResolvedContext: &ResolvedContext{},
		LocatorHints: map[string]map[string]any{}, ResolvedPersona: &Persona{}, InternalTools: []Tool{nil},
		ObservationSink: func(ToolObservation) {},
	})
	if err != nil {
		t.Fatalf("marshal Request: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decoded Request is not valid JSON: %v", err)
	}
	for _, field := range requestFieldWhitelist {
		if _, ok := decoded[field]; !ok {
			t.Errorf("Request whitelist field %q missing from serialization: %s", field, encoded)
		}
	}
	for _, field := range requestFieldBlocklist {
		lower := strings.ToLower(field)
		if _, ok := decoded[lower]; ok {
			t.Errorf("Request leaked non-contract field %q into serialization", field)
		}
	}
}

func TestContractResultSerializationWhitelist(t *testing.T) {
	resultFieldWhitelist := []string{
		"execution_id", "status", "output", "prompt_tokens", "completion_tokens",
		"total_tokens", "tool_call_count", "error_code", "error_message",
	}
	encoded, err := json.Marshal(Result{
		ExecutionID: "exec-1", Status: StatusFailed, Output: "partial",
		PromptTokens: 3, CompletionTokens: 5, TotalTokens: 8, ToolCallCount: 1,
		ErrorCode: "tool_budget", ErrorMessage: "tool call budget exceeded",
	})
	if err != nil {
		t.Fatalf("marshal Result: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decoded Result is not valid JSON: %v", err)
	}
	for _, field := range resultFieldWhitelist {
		if _, ok := decoded[field]; !ok {
			t.Errorf("Result whitelist field %q missing: %s", field, encoded)
		}
	}
}

func TestContractEventCatalog(t *testing.T) {
	// The frozen catalog. A runtime adapter may only emit assistant.*, model.*
	// and tool.* categories; lifecycle and planning events belong to the
	// platform; platform-extension entries are registered families.
	cases := []struct {
		eventType string
		category  ContractEventCategory
		platform  bool // true: a runtime adapter must never emit this type
	}{
		{"execution.started", ContractEventLifecycle, true},
		{"execution.completed", ContractEventLifecycle, true},
		{"execution.failed", ContractEventLifecycle, true},
		{"execution.cancelled", ContractEventLifecycle, true},
		{"execution.plan.resolved", ContractEventPlanning, true},
		{"persona.resolved", ContractEventPlanning, true},
		{"context.loaded", ContractEventPlanning, true},
		{"model.started", ContractEventModel, false},
		{"model.resumed", ContractEventModel, false},
		{"assistant.delta", ContractEventAssistant, false},
		{"assistant.progress", ContractEventAssistant, false},
		{"assistant.completed", ContractEventAssistant, false},
		{"tool.requested", ContractEventTool, false},
		{"tool.started", ContractEventTool, false},
		{"tool.completed", ContractEventTool, false},
		{"tool.failed", ContractEventTool, false},
		{EventContextCompacted, ContractEventMaintenance, false},
		// Registered extension family (I009): platform-owned lifecycle events.
		{"expert_group.resolved", ContractEventPlatformExtension, true},
		{"expert_group.failed", ContractEventPlatformExtension, true},
		// Unknown families stay unclassified and must be rejected by callers.
		{"totally.unknown", ContractEventUnclassifiedEvents, false},
	}
	for _, tc := range cases {
		if got := ContractEventCategoryOf(tc.eventType); got != tc.category {
			t.Errorf("ContractEventCategoryOf(%q) = %q, want %q", tc.eventType, got, tc.category)
		}
	}
}

func TestContractRegisterPlatformLifecycleEventPrefix(t *testing.T) {
	RegisterPlatformLifecycleEventPrefix("feature_flag.")
	defer func() {
		// The prefix list is process-global; keep it small for other tests.
		platformLifecycleEventPrefixes = platformLifecycleEventPrefixes[:len(platformLifecycleEventPrefixes)-1]
	}()
	if got := ContractEventCategoryOf("feature_flag.flipped"); got != ContractEventPlatformExtension {
		t.Fatalf("registered prefix classified as %q, want platform-extension", got)
	}
	// Registering twice must be idempotent so concurrent package init stays safe.
	RegisterPlatformLifecycleEventPrefix("feature_flag.")
}

func TestContractToolLifecycleMatrix(t *testing.T) {
	// One authorized tool call must produce exactly requested -> started ->
	// completed. Every failure path must produce requested -> (started) ->
	// failed with a stable error code. The gateway owns this lifecycle for
	// every runtime, so the matrix below is runtime independent.
	cases := []struct {
		name       string
		call       ToolCall
		tool       Tool
		authorize  ToolAuthorizer
		wantEvents []string
		wantCode   string
		wantErr    bool
	}{
		{
			name:       "authorized call completes",
			call:       ToolCall{ExecutionID: "exec-1", ScopeID: "scope-1", ResourceID: "res-1", Name: "read", Arguments: map[string]any{"q": 1}},
			tool:       ToolFunc{Def: ToolDefinition{Name: "read", Source: "test", ResourceID: "res-1", ReadOnly: true}, Fn: func(context.Context, map[string]any) (ToolResult, error) { return ToolResult{Output: "ok"}, nil }},
			wantEvents: []string{"tool.requested", "tool.started", "tool.completed"},
		},
		{
			name:       "unregistered tool fails with tool_not_found",
			call:       ToolCall{ExecutionID: "exec-1", ScopeID: "scope-1", ResourceID: "res-1", Name: "missing"},
			wantEvents: []string{"tool.requested", "tool.failed"},
			wantCode:   "tool_not_found",
			wantErr:    true,
		},
		{
			name:       "denied tool fails with tool_denied",
			call:       ToolCall{ExecutionID: "exec-1", ScopeID: "scope-1", ResourceID: "res-1", Name: "read"},
			tool:       ToolFunc{Def: ToolDefinition{Name: "read", Source: "test", ResourceID: "res-1"}, Fn: func(context.Context, map[string]any) (ToolResult, error) { return ToolResult{}, nil }},
			authorize:  func(context.Context, ToolCall, ToolDefinition) error { return errors.New("forbidden by policy") },
			wantEvents: []string{"tool.requested", "tool.failed"},
			wantCode:   "tool_denied",
			wantErr:    true,
		},
		{
			name:       "invalid call fails with tool_validation",
			call:       ToolCall{ExecutionID: "exec-1", Name: ""},
			wantEvents: []string{"tool.requested", "tool.failed"},
			wantCode:   "tool_validation",
			wantErr:    true,
		},
		{
			name: "tool implementation error fails with tool_execution",
			call: ToolCall{ExecutionID: "exec-1", ScopeID: "scope-1", ResourceID: "res-1", Name: "read"},
			tool: ToolFunc{Def: ToolDefinition{Name: "read", Source: "test", ResourceID: "res-1"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
				return ToolResult{}, errors.New("upstream unavailable")
			}},
			wantEvents: []string{"tool.requested", "tool.started", "tool.failed"},
			wantCode:   "tool_execution",
			wantErr:    true,
		},
		{
			name: "oversized tool output fails with response_too_large",
			call: ToolCall{ExecutionID: "exec-1", ScopeID: "scope-1", ResourceID: "res-1", Name: "read"},
			tool: ToolFunc{Def: ToolDefinition{Name: "read", Source: "test", ResourceID: "res-1"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
				return ToolResult{Output: strings.Repeat("x", 2048)}, nil
			}},
			wantEvents: []string{"tool.requested", "tool.started", "tool.failed"},
			wantCode:   "response_too_large",
			wantErr:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			registry := NewToolRegistry()
			if tc.tool != nil {
				if err := registry.Register(tc.call.ResourceID, tc.tool); err != nil {
					t.Fatal(err)
				}
			}
			audit := &memoryToolCallStore{}
			gateway := NewPolicyGateway(registry, tc.authorize, time.Second, 1024, 1)
			gateway.AuditStore = audit
			var events []Event
			tc.call.EventSink = func(event Event) error { events = append(events, event); return nil }
			_, err := gateway.Invoke(context.Background(), tc.call)
			if tc.wantErr && err == nil {
				t.Fatalf("expected gateway error, got nil; events=%+v", events)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected gateway error: %v", err)
			}
			var types []string
			for _, event := range events {
				types = append(types, event.Type)
				if ContractEventCategoryOf(event.Type) != ContractEventTool {
					t.Fatalf("gateway emitted non-tool event %q", event.Type)
				}
			}
			if strings.Join(types, ",") != strings.Join(tc.wantEvents, ",") {
				t.Fatalf("event sequence = %v, want %v", types, tc.wantEvents)
			}
			if tc.wantCode != "" {
				// Some failure payloads intentionally omit error_code (the raw
				// message is present); the audit record is the authoritative
				// classification, so accept either source.
				if got := events[len(events)-1].Payload["error_code"]; got != tc.wantCode && audit.records[0].ErrorCode != tc.wantCode {
					t.Fatalf("terminal error_code = %v (audit %q), want %q", got, audit.records[0].ErrorCode, tc.wantCode)
				}
			}
			// Every gateway decision, success or failure, lands in the audit
			// trail. An invalid call with no name may only carry the error
			// classification.
			if len(audit.records) != 1 {
				t.Fatalf("audit trail incomplete: %+v", audit.records)
			}
			if tc.call.Name != "" && audit.records[0].ToolName == "" {
				t.Fatalf("audit trail missing tool name: %+v", audit.records)
			}
			if tc.wantCode != "" && audit.records[0].ErrorCode != tc.wantCode {
				t.Fatalf("audit error_code = %q, want %q", audit.records[0].ErrorCode, tc.wantCode)
			}
		})
	}
}

func TestContractBudgetMatrix(t *testing.T) {
	// The budget matrix freezes which terminal code each budget boundary
	// produces. The platform owns the verdict; a runtime adapter only
	// enforces the platform-provided budget and emits the canonical code.
	cases := []struct {
		name       string
		request    Request
		model      model.LLM
		wantStatus Status
		wantCode   string
	}{
		{
			name:  "iteration budget",
			model: emptyAnswerModel{},
			request: Request{ExecutionID: "exec-iteration", ActorID: "actor-1", ScopeID: "scope-1", Task: "answer",
				Budget: Budget{MaxIterations: 1, MaxToolCalls: 1, MaxTokens: 100, MaxOutputTokens: 100, MaxOutputBytes: 4096}},
			wantStatus: StatusFailed, wantCode: "empty_output",
		},
		{
			name:  "token budget",
			model: tokenBudgetPartialModel{},
			request: Request{ExecutionID: "exec-token", ActorID: "actor-1", ScopeID: "scope-1", Task: "answer",
				Budget: Budget{MaxIterations: 1, MaxToolCalls: 1, MaxTokens: 10, MaxOutputTokens: 100, MaxOutputBytes: 4096}},
			wantStatus: StatusFailed, wantCode: "token_budget",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
				return ModelBuildResult{Client: tc.model, Capabilities: []string{"text"}}, nil
			})
			result, err := runner.Run(context.Background(), tc.request)
			if err == nil {
				t.Fatalf("budget boundary must return an error: %+v", result)
			}
			if result.Status != tc.wantStatus || result.ErrorCode != tc.wantCode {
				t.Fatalf("status/code = %s/%s, want %s/%s", result.Status, result.ErrorCode, tc.wantStatus, tc.wantCode)
			}
		})
	}
}

func TestContractOutputBudgetAndSchemaMatrix(t *testing.T) {
	// The final output budget and OutputSchema validation are enforced by the
	// platform contract, after the agent loop returns.
	cases := []struct {
		name       string
		output     string
		maxBytes   int
		schema     json.RawMessage
		wantStatus Status
		wantCode   string
	}{
		{name: "output within budget", output: "ok", maxBytes: 4096, wantStatus: StatusSucceeded},
		{name: "output over budget", output: strings.Repeat("x", 100), maxBytes: 16, wantStatus: StatusFailed, wantCode: "output_budget"},
		{name: "output violates schema", output: "not-json", maxBytes: 4096, schema: json.RawMessage(`{"type":"object"}`), wantStatus: StatusFailed, wantCode: "agent_output_schema"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := scriptedAnswerModel{text: tc.output}
			runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
				return ModelBuildResult{Client: model, Capabilities: []string{"text", "structured_output"}}, nil
			})
			result, err := runner.Run(context.Background(), Request{
				ExecutionID: "exec-" + tc.name, ActorID: "actor-1", ScopeID: "scope-1", Task: "answer",
				OutputSchema: tc.schema,
				Budget:       Budget{MaxIterations: 1, MaxToolCalls: 1, MaxTokens: 1000, MaxOutputTokens: 4096, MaxOutputBytes: tc.maxBytes},
			})
			if tc.wantStatus == StatusSucceeded {
				if err != nil || result.Status != StatusSucceeded {
					t.Fatalf("result=%+v err=%v, want success", result, err)
				}
				return
			}
			if err == nil || result.Status != tc.wantStatus || result.ErrorCode != tc.wantCode {
				t.Fatalf("result=%+v err=%v, want %s/%s", result, err, tc.wantStatus, tc.wantCode)
			}
		})
	}
}

func TestContractCancellationMatrix(t *testing.T) {
	cases := []struct {
		name       string
		cancel     func(ctx context.Context) context.Context
		wantStatus Status
		wantCode   string
	}{
		{
			name: "caller cancellation",
			cancel: func(ctx context.Context) context.Context {
				nested, cancel := context.WithCancel(ctx)
				cancel()
				return nested
			},
			wantStatus: StatusCancelled, wantCode: "cancelled",
		},
		{
			name: "budget deadline",
			cancel: func(ctx context.Context) context.Context {
				nested, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
				_ = cancel
				return nested
			},
			wantStatus: StatusCancelled, wantCode: "timeout",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := blockingModel{}
			runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
				return ModelBuildResult{Client: model, Capabilities: []string{"text"}}, nil
			})
			result, err := runner.Run(tc.cancel(context.Background()), Request{
				ExecutionID: "exec-cancel-" + tc.name, ActorID: "actor-1", ScopeID: "scope-1", Task: "answer",
				Budget: Budget{MaxIterations: 2, MaxToolCalls: 2, MaxTokens: 1000, MaxOutputBytes: 4096},
			})
			if err == nil || result.Status != tc.wantStatus || result.ErrorCode != tc.wantCode {
				t.Fatalf("result=%+v err=%v, want %s/%s", result, err, tc.wantStatus, tc.wantCode)
			}
		})
	}
}

func TestContractCapabilityMatrix(t *testing.T) {
	// missingCapabilities is the model capability check; CapabilitySet.Missing
	// is the runtime adapter form. Both must agree that unknown requirements
	// are reported, not silently accepted.
	cases := []struct {
		name     string
		actual   []string
		required []string
		want     []string
	}{
		{name: "satisfied", actual: []string{"text", "tool_calling", "stream"}, required: []string{"tool_calling", "stream"}, want: nil},
		{name: "missing stream", actual: []string{"text"}, required: []string{"stream"}, want: []string{"stream"}},
		{name: "case and space insensitive", actual: []string{"text"}, required: []string{" stream "}, want: []string{"stream"}},
		{name: "no duplicates", actual: nil, required: []string{"a", "a"}, want: []string{"a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := missingCapabilities(tc.actual, tc.required); strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("missingCapabilities = %v, want %v", got, tc.want)
			}
			if got := CapabilitySet(tc.actual).Missing(tc.required); strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("CapabilitySet.Missing = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestContractAdapterRequestValidation(t *testing.T) {
	cases := []struct {
		name    string
		request AdapterRequest
		wantErr string
	}{
		{name: "valid", request: AdapterRequest{ScopeID: "scope-1", Task: "answer"}, wantErr: ""},
		{name: "missing scope", request: AdapterRequest{Task: "answer"}, wantErr: "scope_id is required"},
		{name: "missing prompt", request: AdapterRequest{ScopeID: "scope-1"}, wantErr: "prompt, task or messages is required"},
		{name: "negative budget", request: AdapterRequest{ScopeID: "scope-1", Task: "answer", Budget: Budget{MaxTokens: -1}}, wantErr: "budget must not be negative"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.request.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

type contractFakeAdapter struct {
	id      string
	caps    CapabilitySet
	version string
}

func (a contractFakeAdapter) ID() string                  { return a.id }
func (a contractFakeAdapter) Version() string             { return a.version }
func (a contractFakeAdapter) Capabilities() CapabilitySet { return a.caps }
func (a contractFakeAdapter) Execute(context.Context, AdapterRequest, AdapterDeps) (AdapterResult, error) {
	return AdapterResult{Status: StatusSucceeded}, nil
}
func (a contractFakeAdapter) Stream(context.Context, AdapterRequest, AdapterDeps) (AdapterResult, error) {
	return AdapterResult{Status: StatusSucceeded}, nil
}

func TestContractValidateAdapterRequestCapabilityGate(t *testing.T) {
	adapter := contractFakeAdapter{id: "fake", version: "v1", caps: CapabilitySet{AdapterCapabilityAgentLoop, AdapterCapabilityToolCalling}}
	cases := []struct {
		name    string
		request AdapterRequest
		wantErr string
	}{
		{name: "tool request satisfied", request: AdapterRequest{ScopeID: "scope-1", Task: "answer", Requirements: Requirements{Capabilities: []string{"tool_calling"}}}},
		{name: "streaming enforced", request: AdapterRequest{ScopeID: "scope-1", Task: "answer", Stream: true}, wantErr: "lacks capabilities: stream"},
		{name: "structured output enforced", request: AdapterRequest{ScopeID: "scope-1", Task: "answer", Requirements: Requirements{StructuredOut: true}}, wantErr: "lacks capabilities: structured_output"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAdapterRequest(tc.request, adapter)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
	if err := ValidateAdapterRequest(AdapterRequest{ScopeID: "scope-1", Task: "answer"}, nil); err == nil {
		t.Fatal("nil adapter must be rejected")
	}
}

func TestContractAdapterRegistry(t *testing.T) {
	registry := NewAdapterRegistry()
	if err := registry.Register(contractFakeAdapter{id: "adk", version: "v2.2.0", caps: CapabilitySet{AdapterCapabilityAgentLoop}}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(contractFakeAdapter{id: "pi", version: "1.5.2", caps: CapabilitySet{AdapterCapabilityAgentLoop, AdapterCapabilityStreaming}}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(nil); err == nil {
		t.Fatal("nil adapter must be rejected")
	}
	if err := registry.Register(contractFakeAdapter{id: "  ", version: "v1"}); err == nil {
		t.Fatal("empty adapter id must be rejected")
	}
	if err := registry.Register(contractFakeAdapter{id: "noversion"}); err == nil {
		t.Fatal("missing adapter version must be rejected")
	}
	if adapter, ok := registry.Get("adk"); !ok || adapter.Version() != "v2.2.0" {
		t.Fatalf("registry lookup failed: %+v %v", adapter, ok)
	}
	if _, ok := registry.Get("missing"); ok {
		t.Fatal("unknown adapter id must not resolve")
	}
	ids := make([]string, 0)
	for _, adapter := range registry.List() {
		ids = append(ids, adapter.ID())
	}
	if strings.Join(ids, ",") != "adk,pi" {
		t.Fatalf("registry list = %v, want deterministic id order", ids)
	}
}

func TestContractModelDescriptionIsCredentialFree(t *testing.T) {
	// The framework-neutral model description must carry everything an
	// adapter needs to build a client and nothing that could persist a
	// credential (FR-004, ai-engine.md 19.4).
	build := ModelBuildResult{
		ProviderID: "provider-1", ModelName: "model-a",
		Capabilities: []string{"text", "stream"}, ContextWindowTokens: 128000,
		MaxOutputTokens: 8192, Temperature: 0.3, Protocol: ModelProtocolOpenAICompatible,
	}
	description := build.ModelDescription()
	if description.ProviderID != "provider-1" || description.ModelName != "model-a" {
		t.Fatalf("description identity = %+v", description)
	}
	if description.Protocol != ModelProtocolOpenAICompatible || description.ContextWindowTokens != 128000 || description.MaxOutputTokens != 8192 || description.Temperature != 0.3 {
		t.Fatalf("description limits = %+v", description)
	}
	if !CapabilitySet(description.Capabilities).Has("stream") {
		t.Fatalf("description capabilities = %+v", description.Capabilities)
	}
	// Credential fields are json:"-"; the description has no credential
	// fields at all. A credential value must never survive in it.
	credential := ModelCredential{APIKey: "sk-secret", BaseURL: "https://internal", Headers: map[string]string{"x-key": "v"}}
	if credential.Empty() {
		t.Fatal("populated credential reported empty")
	}
	if (ModelCredential{}).Empty() != true {
		t.Fatal("zero credential must report empty")
	}
	encoded, err := json.Marshal(description)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "sk-secret") {
		t.Fatalf("credential leaked into model description: %s", encoded)
	}
}

func TestContractToolInvokerFunc(t *testing.T) {
	invoked := false
	invoker := ToolInvokerFunc(func(_ context.Context, call ToolCall) (ToolResult, error) {
		invoked = call.Name == "read"
		return ToolResult{Output: "ok"}, nil
	})
	result, err := invoker.InvokeTool(context.Background(), ToolCall{Name: "read"})
	if err != nil || !invoked || result.Output != "ok" {
		t.Fatalf("invoker result=%+v err=%v invoked=%v", result, err, invoked)
	}
	var nilInvoker ToolInvokerFunc
	if _, err := nilInvoker.InvokeTool(context.Background(), ToolCall{}); !errors.Is(err, ErrToolInvalid) {
		t.Fatalf("nil invoker error = %v, want ErrToolInvalid", err)
	}
}
