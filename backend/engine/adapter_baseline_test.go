package engine

import (
	"context"
	"errors"
	"iter"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// This file records the ADK behavior baseline snapshot (I008 A2-2, R001
// FR-002). Each scenario drives the current AgentRunner (the ADK-backed
// runtime) with a deterministic Mock Model and a tool fixture, then freezes
// the exact event-type sequence. The B3 ADK adapter refactor and any later
// runtime (pi) must reproduce these sequences without changing the test
// semantics. Later iterations may append new event types (platform-extension
// categories); the assertion helper tolerates insertions of platform-extension
// events only, everything else must match exactly.

// baselineEventTypes reduces captured events to their type sequence.
func baselineEventTypes(events []Event) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}

// assertBaselineSequence compares the recorded sequence with the frozen
// baseline. Events whose category is platform-extension (registered by later
// iterations, for example I009) are ignored, so registering a new lifecycle
// family never invalidates this baseline.
func assertBaselineSequence(t *testing.T, name string, got []string, want []string) {
	t.Helper()
	filtered := make([]string, 0, len(got))
	for _, eventType := range got {
		if ContractEventCategoryOf(eventType) == ContractEventPlatformExtension {
			continue
		}
		filtered = append(filtered, eventType)
	}
	if strings.Join(filtered, ",") != strings.Join(want, ",") {
		t.Fatalf("baseline %q drifted:\n got: %v\nwant: %v", name, filtered, want)
	}
}

// baselineFixture builds the standard tool fixture used by every baseline
// scenario: one gateway-backed resource with the named tools.
func baselineFixture(t *testing.T, tools ...ToolFunc) (*PolicyGateway, *memoryToolCallStore) {
	t.Helper()
	registry := NewToolRegistry()
	for _, tool := range tools {
		if err := registry.Register(tool.Def.ResourceID, tool); err != nil {
			t.Fatal(err)
		}
	}
	audit := &memoryToolCallStore{}
	gateway := NewPolicyGateway(registry, nil, 5*time.Second, 64<<10, 4)
	gateway.AuditStore = audit
	return gateway, audit
}

// scriptedAnswerModel answers with fixed text on every call.
type scriptedAnswerModel struct{ text string }

func (m scriptedAnswerModel) Name() string { return "scripted-answer-model" }

func (m scriptedAnswerModel) GenerateContent(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		yield(&model.LLMResponse{Content: genai.NewContentFromText(m.text, genai.RoleModel)}, nil)
	}
}

// blockingModel never answers before the execution context is cancelled; it
// forwards the context error to the runner.
type blockingModel struct{}

func (blockingModel) Name() string { return "blocking-model" }

func (blockingModel) GenerateContent(ctx context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		<-ctx.Done()
		yield(nil, ctx.Err())
	}
}

// baselineToolCallModel performs one ReAct round: narration, one tool call on
// turn one, then a final answer that quotes the tool observation.
type baselineToolCallModel struct {
	mu    sync.Mutex
	turns int
	quote string
}

func (m *baselineToolCallModel) Name() string { return "baseline-tool-call-model" }

func (m *baselineToolCallModel) GenerateContent(_ context.Context, request *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		m.mu.Lock()
		m.turns++
		turn := m.turns
		m.mu.Unlock()
		if turn == 1 {
			if stream {
				for _, text := range []string{"先", "检查", "资源。"} {
					if !yield(&model.LLMResponse{Content: genai.NewContentFromText(text, genai.RoleModel), Partial: true}, nil) {
						return
					}
				}
			}
			yield(&model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{
				{Text: "先检查资源。"},
				{FunctionCall: &genai.FunctionCall{ID: "baseline-call-1", Name: "lookup", Args: map[string]any{"name": "web"}}},
			}}}, nil)
			return
		}
		yield(&model.LLMResponse{Content: genai.NewContentFromText("已根据观察完成检查。", genai.RoleModel)}, nil)
	}
}

func TestADKBaselineHappyReActStream(t *testing.T) {
	gateway, audit := baselineFixture(t, ToolFunc{
		Def: ToolDefinition{Name: "lookup", Source: "test", ResourceID: "baseline-res-1"},
		Fn: func(context.Context, map[string]any) (ToolResult, error) {
			return ToolResult{Output: map[string]any{"status": "running"}}, nil
		},
	})
	modelClient := &baselineToolCallModel{}
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: modelClient, Capabilities: []string{"text", "tool_calling", "stream"}}, nil
	})
	var events []Event
	result, err := runner.RunStream(context.Background(), Request{
		ExecutionID: "baseline-react", ActorID: "actor-1", ScopeID: "scope-1", Task: "check", Stream: true,
		Budget:          Budget{MaxIterations: 4, MaxToolCalls: 4, MaxTokens: 1000, MaxOutputTokens: 4096, MaxOutputBytes: 4096},
		ResolvedContext: &ResolvedContext{Tools: []ToolDefinition{{Name: "lookup", Source: "test", ResourceID: "baseline-res-1"}}},
		ToolGateway:     gateway,
	}, func(event Event) error { events = append(events, event); return nil })
	if err != nil || result.Status != StatusSucceeded || result.Output != "已根据观察完成检查。" || result.ToolCallCount != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	want := []string{
		"model.started",
		"assistant.delta", "assistant.delta", "assistant.delta",
		"assistant.progress",
		"tool.requested", "tool.started", "tool.completed",
		"model.resumed",
		"assistant.progress",
		"model.started",
		"assistant.completed",
	}
	assertBaselineSequence(t, "happy-react-stream", baselineEventTypes(events), want)
	if len(audit.records) != 1 || audit.records[0].Status != StatusSucceeded {
		t.Fatalf("audit trail = %+v", audit.records)
	}
}

func TestADKBaselineToolFailureCorrection(t *testing.T) {
	gateway, audit := baselineFixture(t,
		ToolFunc{Def: ToolDefinition{Name: "failing_lookup", Source: "test", ResourceID: "baseline-res-1"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
			return ToolResult{}, errors.New("upstream unavailable")
		}},
		ToolFunc{Def: ToolDefinition{Name: "fallback_lookup", Source: "test", ResourceID: "baseline-res-1"}, Fn: func(context.Context, map[string]any) (ToolResult, error) {
			return ToolResult{Output: map[string]any{"ok": true}}, nil
		}},
	)
	modelClient := &correctionTestModel{}
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: modelClient, Capabilities: []string{"text", "tool_calling", "stream"}}, nil
	})
	var events []Event
	result, err := runner.RunStream(context.Background(), Request{
		ExecutionID: "baseline-correction", ActorID: "actor-1", ScopeID: "scope-1", Task: "diagnose", Stream: true,
		Budget:          Budget{MaxIterations: 5, MaxToolCalls: 5, MaxTokens: 1000, MaxOutputTokens: 4096, MaxOutputBytes: 4096},
		ResolvedContext: &ResolvedContext{Tools: []ToolDefinition{{Name: "failing_lookup", Source: "test", ResourceID: "baseline-res-1"}, {Name: "fallback_lookup", Source: "test", ResourceID: "baseline-res-1"}}},
		ToolGateway:     gateway,
	}, func(event Event) error { events = append(events, event); return nil })
	if err != nil || result.Output != "备用查询成功，已完成诊断。" || result.ToolCallCount != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	want := []string{
		"model.started",
		"assistant.progress",
		"tool.requested", "tool.started", "tool.failed",
		"model.resumed", "assistant.progress",
		"model.started",
		"assistant.progress",
		"tool.requested", "tool.started", "tool.completed",
		"model.resumed", "assistant.progress",
		"model.started",
		"assistant.completed",
	}
	assertBaselineSequence(t, "tool-failure-correction", baselineEventTypes(events), want)
	if len(audit.records) != 2 || audit.records[0].Status != StatusFailed || audit.records[1].Status != StatusSucceeded {
		t.Fatalf("audit trail = %+v", audit.records)
	}
}

func TestADKBaselineUnknownToolRequest(t *testing.T) {
	gateway, audit := baselineFixture(t, ToolFunc{
		Def: ToolDefinition{Name: "known", Source: "test", ResourceID: "baseline-res-1"},
		Fn: func(context.Context, map[string]any) (ToolResult, error) {
			return ToolResult{Output: map[string]any{"ok": true}}, nil
		},
	})
	modelClient := &unknownToolModel{}
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: modelClient, Capabilities: []string{"text", "tool_calling", "stream"}}, nil
	})
	var events []Event
	result, err := runner.RunStream(context.Background(), Request{
		ExecutionID: "baseline-unknown-tool", ActorID: "actor-1", ScopeID: "scope-1", Task: "answer", Stream: true,
		Budget:          Budget{MaxIterations: 3, MaxToolCalls: 3, MaxTokens: 1000, MaxOutputTokens: 4096, MaxOutputBytes: 4096},
		ResolvedContext: &ResolvedContext{Tools: []ToolDefinition{{Name: "known", Source: "test", ResourceID: "baseline-res-1"}}},
		ToolGateway:     gateway,
	}, func(event Event) error { events = append(events, event); return nil })
	if err != nil || result.Output != "工具不存在，已改为直接说明限制。" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	want := []string{
		"model.started",
		"assistant.progress",
		"tool.requested", "tool.failed",
		"model.resumed", "assistant.progress",
		"model.started",
		"assistant.completed",
	}
	assertBaselineSequence(t, "unknown-tool", baselineEventTypes(events), want)
	if len(audit.records) != 1 || audit.records[0].ErrorCode != "tool_not_found" || audit.records[0].ToolName != "not_registered" {
		t.Fatalf("unknown tool audit = %+v", audit.records)
	}
}

func TestADKBaselineTokenBudget(t *testing.T) {
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: tokenBudgetPartialModel{}, Capabilities: []string{"text"}}, nil
	})
	var events []Event
	result, err := runner.Run(context.Background(), Request{
		ExecutionID: "baseline-token-budget", ActorID: "actor-1", ScopeID: "scope-1", Task: "answer",
		Budget:    Budget{MaxIterations: 1, MaxToolCalls: 1, MaxTokens: 10, MaxOutputTokens: 4096, MaxOutputBytes: 4096},
		EventSink: func(event Event) error { events = append(events, event); return nil },
	})
	if err == nil || result.Status != StatusFailed || result.ErrorCode != "token_budget" || result.Output != "预算前已生成的回答" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	want := []string{
		"model.started",
		"phase.changed",
	}
	assertBaselineSequence(t, "token-budget", baselineEventTypes(events), want)
}

func TestADKBaselineCancellation(t *testing.T) {
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: blockingModel{}, Capabilities: []string{"text"}}, nil
	})
	var events []Event
	result, err := runner.Run(context.Background(), Request{
		ExecutionID: "baseline-cancel", ActorID: "actor-1", ScopeID: "scope-1", Task: "answer",
		Budget:    Budget{MaxIterations: 2, MaxToolCalls: 2, MaxTokens: 1000, MaxOutputTokens: 4096, MaxOutputBytes: 4096, Timeout: 100 * time.Millisecond},
		EventSink: func(event Event) error { events = append(events, event); return nil },
	})
	if err == nil || result.Status != StatusCancelled || result.ErrorCode != "timeout" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	want := []string{"model.started"}
	assertBaselineSequence(t, "cancellation", baselineEventTypes(events), want)
}

func TestADKBaselineRuntimeLifecycleBracketing(t *testing.T) {
	// The platform Runtime brackets every adapter with the frozen lifecycle
	// events: execution.started opens, exactly one terminal event closes.
	// This is platform-owned and must never be emitted by a runtime adapter.
	gateway, _ := baselineFixture(t)
	modelClient := &baselineToolCallModel{}
	runner := NewAgentRunner(func(context.Context, string, string, string, Purpose) (ModelBuildResult, error) {
		return ModelBuildResult{Client: modelClient, Capabilities: []string{"text", "tool_calling"}}, nil
	})
	store := &fakeEventStore{}
	runtime := NewWithContextAndStore(runner, nil, gateway, store)
	result, err := runtime.Execute(context.Background(), Request{
		ExecutionID: "baseline-lifecycle", ActorID: "actor-1", ScopeID: "scope-1", Task: "check",
		Budget:          Budget{MaxIterations: 4, MaxToolCalls: 4, MaxTokens: 1000, MaxOutputTokens: 4096, MaxOutputBytes: 4096},
		ResolvedContext: &ResolvedContext{Tools: []ToolDefinition{{Name: "lookup", Source: "test", ResourceID: "baseline-res-1"}}},
	})
	if err != nil || result.Status != StatusSucceeded {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	types := baselineEventTypes(store.events)
	if types[0] != "execution.started" {
		t.Fatalf("first persisted event = %q, want execution.started", types[0])
	}
	if types[len(types)-1] != "execution.completed" {
		t.Fatalf("last persisted event = %q, want execution.completed", types[len(types)-1])
	}
	terminal := 0
	for _, eventType := range types {
		switch eventType {
		case "execution.completed", "execution.failed", "execution.cancelled":
			terminal++
		}
	}
	if terminal != 1 {
		t.Fatalf("terminal events = %d, want exactly one", terminal)
	}
	for index := 1; index < len(store.events); index++ {
		if store.events[index].Sequence <= store.events[index-1].Sequence {
			t.Fatalf("persisted sequence is not monotonic: %+v", store.events)
		}
	}
	// Sequence numbers are assigned by the platform, dense from 1.
	if store.events[0].Sequence != 1 {
		t.Fatalf("first sequence = %d, want 1", store.events[0].Sequence)
	}
}
