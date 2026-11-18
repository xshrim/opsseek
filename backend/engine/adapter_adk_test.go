package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// These tests pin the ADK runtime adapter surface (I008 B3, R001 FR-003):
// registry metadata, the AdapterRequest -> platform Request bridge, the tool
// gateway routing through AdapterDeps.Tool/Audit and the runtime factory
// fallback behavior. The full behavioral baseline stays in
// adapter_baseline_test.go; those scenarios must keep passing unchanged.

func adkTestAdapter(t *testing.T, capabilities []string) *ADKAdapter {
	t.Helper()
	if capabilities == nil {
		capabilities = []string{"text", "tool_calling", "stream"}
	}
	return NewADKAdapter(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: &baselineToolCallModel{}, Capabilities: capabilities}, nil
	})
}

func TestADKAdapterRegistryMetadata(t *testing.T) {
	adapter := NewADKAdapter(nil)
	if adapter.ID() != "adk" {
		t.Fatalf("adapter id = %q, want adk", adapter.ID())
	}
	if strings.TrimSpace(adapter.Version()) == "" {
		t.Fatal("adapter version must not be empty")
	}
	if missing := adapter.Capabilities().Missing([]string{
		AdapterCapabilityAgentLoop, AdapterCapabilityToolCalling,
		AdapterCapabilityStreaming, AdapterCapabilityStructuredOutput,
		AdapterCapabilityContextCompaction,
	}); len(missing) > 0 {
		t.Fatalf("adapter lacks capabilities: %v", missing)
	}
	// The adapter must satisfy the frozen contract end to end.
	var _ EngineAdapter = (*ADKAdapter)(nil)
	var contractAdapter EngineAdapter = adapter
	if _, ok := contractAdapter.(RunnerProvider); !ok {
		t.Fatal("ADKAdapter must expose its platform runner")
	}
	registry, err := DefaultAdapterRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	if resolved, ok := registry.Get("adk"); !ok || resolved.ID() != "adk" {
		t.Fatalf("default registry did not resolve the adk adapter: %+v %v", resolved, ok)
	}
}

func TestADKAdapterExecuteToolRoundThroughGateway(t *testing.T) {
	adapter := adkTestAdapter(t, nil)
	var events []Event
	auditRecords := make([]ToolCallRecord, 0)
	deps := AdapterDeps{
		Tool: ToolInvokerFunc(func(_ context.Context, call ToolCall) (ToolResult, error) {
			if call.Name != "lookup" {
				return ToolResult{}, errors.New("unexpected tool " + call.Name)
			}
			if call.ScopeID != "scope-1" || call.ActorID != "actor-1" {
				return ToolResult{}, errors.New("adapter lost the authorized identity")
			}
			return ToolResult{Output: map[string]any{"status": "running"}}, nil
		}),
		Emit: func(event Event) error { events = append(events, event); return nil },
		Audit: func(_ context.Context, record ToolCallRecord) error {
			auditRecords = append(auditRecords, record)
			return nil
		},
	}
	result, err := adapter.Execute(context.Background(), AdapterRequest{
		ExecutionID: "adapter-exec-1", ActorID: "actor-1", ScopeID: "scope-1", Task: "check",
		Budget: Budget{MaxIterations: 4, MaxToolCalls: 4, MaxTokens: 1000, MaxOutputTokens: 4096, MaxOutputBytes: 4096},
		Tools:  []ToolDeclaration{{Name: "lookup", Description: "look up a resource"}},
	}, deps)
	if err != nil || result.Status != StatusSucceeded || result.ToolCallCount != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertBaselineSequence(t, "adapter-execute-tool", baselineEventTypes(events), []string{
		"model.started",
		"assistant.progress",
		"tool.requested", "tool.started", "tool.completed",
		"model.resumed",
		"assistant.progress",
		"model.started",
		"assistant.completed",
	})
	if len(auditRecords) != 1 || auditRecords[0].ToolName != "lookup" || auditRecords[0].Status != StatusSucceeded {
		t.Fatalf("adapter audit trail = %+v", auditRecords)
	}
}

func TestADKAdapterStreamEmitsDeltas(t *testing.T) {
	adapter := adkTestAdapter(t, nil)
	var events []Event
	deps := AdapterDeps{Emit: func(event Event) error { events = append(events, event); return nil }}
	result, err := adapter.Stream(context.Background(), AdapterRequest{
		ExecutionID: "adapter-stream-1", ActorID: "actor-1", ScopeID: "scope-1", Task: "check", Stream: true,
		Budget: Budget{MaxIterations: 4, MaxToolCalls: 4, MaxTokens: 1000, MaxOutputTokens: 4096, MaxOutputBytes: 4096},
	}, deps)
	if err != nil || result.Status != StatusSucceeded {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	sawDelta := false
	for _, event := range events {
		if event.Type == "assistant.delta" {
			sawDelta = true
		}
		if ContractEventCategoryOf(event.Type) == ContractEventUnclassifiedEvents {
			t.Fatalf("adapter emitted unclassified event %q", event.Type)
		}
	}
	if !sawDelta {
		t.Fatalf("streaming execution produced no assistant.delta: %+v", events)
	}
}

func TestADKAdapterExecuteRejectsInvalidSnapshot(t *testing.T) {
	adapter := adkTestAdapter(t, nil)
	if _, err := adapter.Execute(context.Background(), AdapterRequest{Task: "answer"}, AdapterDeps{}); err == nil {
		t.Fatal("adapter must reject a request without scope")
	}
	// A streaming request against an adapter without the stream capability
	// must be rejected by the shared capability gate.
	limited := adkTestAdapter(t, []string{"text"})
	// capabilities come from the model builder; emulate the gate directly.
	if err := ValidateAdapterRequest(AdapterRequest{ScopeID: "scope-1", Task: "answer", Stream: true}, contractFakeAdapter{id: "nostream", version: "v1", caps: CapabilitySet{AdapterCapabilityAgentLoop}}); err == nil {
		t.Fatal("stream requirement must be enforced")
	}
	_ = limited
}

func TestResolveEngineRunnerFactory(t *testing.T) {
	registry, err := DefaultAdapterRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	runner, adapter, note := ResolveEngineRunner(registry, "adk", nil)
	if runner == nil || adapter.ID() != "adk" || note != "" {
		t.Fatalf("explicit adk resolution failed: runner=%v adapter=%v note=%q", runner, adapter, note)
	}
	runner, adapter, note = ResolveEngineRunner(registry, "", nil)
	if runner == nil || adapter.ID() != "adk" || note != "" {
		t.Fatalf("empty runtime kind must resolve the default silently: note=%q", note)
	}
	runner, adapter, note = ResolveEngineRunner(registry, "pi-someday", nil)
	if runner == nil || adapter.ID() != "adk" {
		t.Fatalf("unknown runtime kind must fall back to adk: adapter=%v", adapter)
	}
	if !strings.Contains(note, `"pi-someday" is not registered`) {
		t.Fatalf("fallback note must record the trace: %q", note)
	}
}
