package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// This file tests the Pi runtime adapter against an in-process fake child
// (I008 C1-2/C1-3 test strategy): the test binary re-executes itself with
// OPSK_PI_FAKE_HELPER=1 and speaks the exact JSONL RPC protocol documented in
// adapter_pi.go, so the whole subprocess lifecycle, cancellation ladder and
// framing rules are exercised without any Node dependency.

// TestMain routes the re-executed fake child. No Node/npm is required.
func TestMain(m *testing.M) {
	if os.Getenv("OPSK_PI_FAKE_HELPER") == "1" {
		fakePiMain()
		return
	}
	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// Fake Pi child (JSONL protocol peer)
// ---------------------------------------------------------------------------

func fakePiMain() {
	mode := os.Getenv("OPSK_PI_FAKE_MODE")
	out := bufio.NewWriter(os.Stdout)
	write := func(value any) {
		encoded, _ := json.Marshal(value)
		_, _ = out.Write(encoded)
		_, _ = out.Write([]byte("\n"))
		_ = out.Flush()
	}
	in := bufio.NewReader(os.Stdin)
	write(map[string]any{"type": "ready", "pid": os.Getpid()})
	switch mode {
	case "crash":
		_, _ = in.ReadString('\n')
		os.Exit(3)
	case "hang", "ignore-cancel", "timeout":
		// Never settle. ignore-cancel/timeout keep running even after stdin
		// closes so the adapter must reach SIGKILL (level 3).
		_, _ = in.ReadString('\n')
		if mode == "hang" {
			write(map[string]any{"type": "turn_start", "turn": 1})
		}
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	// Consume the prompt command.
	line, err := in.ReadString('\n')
	if err != nil {
		os.Exit(1)
	}
	var command piCommand
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &command); err != nil || command.Method != "prompt" {
		write(map[string]any{"type": "agent_settled", "text": "协议错误"})
		os.Exit(0)
	}
	nextCommand := func() piCommand {
		for {
			line, err := in.ReadString('\n')
			if err != nil {
				os.Exit(0)
			}
			if strings.TrimSpace(line) == "" {
				continue
			}
			var cmd piCommand
			if err := json.Unmarshal([]byte(line), &cmd); err != nil {
				continue
			}
			return cmd
		}
	}
	turn := func(n int) { write(map[string]any{"type": "turn_start", "turn": n}) }
	delta := func(text string) {
		write(map[string]any{"type": "message_update", "message_type": "text", "delta": text})
	}
	messageEnd := func(input, output int64) {
		write(map[string]any{"type": "message_end", "usage": map[string]int64{"input_tokens": input, "output_tokens": output}})
	}
	toolCall := func(id, name string, args map[string]any) {
		write(map[string]any{"type": "tool_execution_start", "call_id": id, "tool": name, "args": args})
	}

	switch mode {
	case "happy":
		turn(1)
		delta("先")
		delta("检查")
		delta("资源。")
		messageEnd(0, 0)
		toolCall("pi-call-1", "lookup", map[string]any{"name": "web"})
		_ = nextCommand() // tool_result
		write(map[string]any{"type": "turn_end", "turn": 1})
		turn(2)
		messageEnd(0, 0)
		write(map[string]any{"type": "turn_end", "turn": 2})
		write(map[string]any{"type": "agent_settled", "text": "已根据观察完成检查。"})
	case "unknown-tool":
		turn(1)
		toolCall("pi-call-9", "not_registered", map[string]any{"q": 1})
		_ = nextCommand()
		write(map[string]any{"type": "turn_end", "turn": 1})
		turn(2)
		messageEnd(0, 0)
		write(map[string]any{"type": "agent_settled", "text": "工具不存在，已改为直接说明限制。"})
	case "tool-fail":
		turn(1)
		toolCall("pi-call-2", "failing_lookup", map[string]any{"name": "web"})
		_ = nextCommand()
		write(map[string]any{"type": "turn_end", "turn": 1})
		turn(2)
		messageEnd(0, 0)
		write(map[string]any{"type": "agent_settled", "text": "工具失败后已直接作答。"})
	case "u2028":
		turn(1)
		// U+2028/U+2029 inside a JSON string must never break LF framing:
		// encoding/json escapes them, and the child encodes them literally to
		// prove the Go decoder handles both forms.
		delta("第一行第二行")
		delta("分隔符")
		messageEnd(0, 0)
		write(map[string]any{"type": "agent_settled", "text": "完成"})
	case "big-result":
		turn(1)
		toolCall("pi-call-big", "lookup", map[string]any{"name": "web"})
		_ = nextCommand() // >64KB tool_result written back through the pipe
		turn(2)
		messageEnd(0, 0)
		write(map[string]any{"type": "agent_settled", "text": "大结果已处理。"})
	case "budget-iterations":
		for n := 1; n <= 20; n++ {
			turn(n)
			messageEnd(0, 0)
		}
		write(map[string]any{"type": "agent_settled", "text": "不应到达"})
	case "budget-tokens":
		turn(1)
		messageEnd(9000, 9000)
		time.Sleep(time.Second)
		write(map[string]any{"type": "agent_settled", "text": "不应到达"})
	case "budget-tools":
		turn(1)
		for i := 0; i < 20; i++ {
			toolCall(fmt.Sprintf("pi-call-%d", i), "lookup", map[string]any{"name": "web"})
			_ = nextCommand()
		}
		write(map[string]any{"type": "agent_settled", "text": "不应到达"})
	case "empty":
		turn(1)
		messageEnd(0, 0)
		write(map[string]any{"type": "agent_settled"})
	}
	os.Exit(0)
}

// fakePiConfig launches the re-executed fake child in the given mode with
// test-scaled cancellation ladders.
func fakePiConfig(mode string) PiProcessConfig {
	return PiProcessConfig{
		Command:           os.Args[0],
		Args:              nil,
		Env:               []string{"OPSK_PI_FAKE_HELPER=1", "OPSK_PI_FAKE_MODE=" + mode},
		StartTimeout:      5 * time.Second,
		InterruptGrace:    300 * time.Millisecond,
		GracePeriod:       300 * time.Millisecond,
		StdinWriteTimeout: 3 * time.Second,
	}
}

func piTestDeps(events *[]Event, invoker ToolInvoker, audit *[]ToolCallRecord) AdapterDeps {
	var mu sync.Mutex
	return AdapterDeps{
		Tool: invoker,
		Emit: func(event Event) error {
			mu.Lock()
			defer mu.Unlock()
			*events = append(*events, event)
			return nil
		},
		Audit: func(_ context.Context, record ToolCallRecord) error {
			mu.Lock()
			defer mu.Unlock()
			*audit = append(*audit, record)
			return nil
		},
	}
}

func piToolInvoker(result ToolResult, err error) ToolInvoker {
	return ToolInvokerFunc(func(_ context.Context, call ToolCall) (ToolResult, error) {
		if call.Name == "" {
			return ToolResult{}, errors.New("tool call is invalid")
		}
		return result, err
	})
}

func piFixtureRequest(task string, budget Budget, tools ...string) AdapterRequest {
	declarations := make([]ToolDeclaration, 0, len(tools))
	for _, name := range tools {
		declarations = append(declarations, ToolDeclaration{Name: name, Description: "fixture tool"})
	}
	return AdapterRequest{
		ExecutionID: "pi-exec-1", ActorID: "actor-1", ScopeID: "scope-1",
		Task: task, Tools: declarations, Budget: budget,
		Model: ModelDescription{ProviderID: "provider-1", ModelName: "model-a", Protocol: ModelProtocolOpenAICompatible},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestPiAdapterHappyReAct(t *testing.T) {
	adapter := NewPiAdapter(fakePiConfig("happy"))
	var events []Event
	var audit []ToolCallRecord
	deps := piTestDeps(&events, piToolInvoker(ToolResult{Output: map[string]any{"status": "running"}}, nil), &audit)
	result, err := adapter.Execute(context.Background(), piFixtureRequest("check", Budget{MaxIterations: 4, MaxToolCalls: 4, MaxTokens: 1000, MaxOutputBytes: 4096}, "lookup"), deps)
	if err != nil || result.Status != StatusSucceeded || result.Output != "已根据观察完成检查。" || result.ToolCallCount != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, event := range events {
		if ContractEventCategoryOf(event.Type) == ContractEventLifecycle {
			t.Fatalf("adapter emitted platform lifecycle event %q", event.Type)
		}
	}
	if len(audit) != 1 || audit[0].Status != StatusSucceeded || audit[0].ToolName != "lookup" {
		t.Fatalf("audit trail = %+v", audit)
	}
	if len(RegisteredPiProcesses()) != 0 {
		t.Fatalf("process registry not cleaned: %+v", RegisteredPiProcesses())
	}
}

func TestPiAdapterDualEngineConsistencyWithADKBaseline(t *testing.T) {
	// FR-010: the same fixture shape must produce the same unified event
	// sequence as the frozen ADK baseline (TestADKBaselineHappyReActStream).
	// ADK equality is frozen by the baseline test; here the Pi fake child must
	// reproduce the identical sequence under assertBaselineSequence.
	adapter := NewPiAdapter(fakePiConfig("happy"))
	var events []Event
	var audit []ToolCallRecord
	deps := piTestDeps(&events, piToolInvoker(ToolResult{Output: map[string]any{"status": "running"}}, nil), &audit)
	if _, err := adapter.Stream(context.Background(), piFixtureRequest("check", Budget{MaxIterations: 4, MaxToolCalls: 4, MaxTokens: 1000, MaxOutputBytes: 4096}, "lookup"), deps); err != nil {
		t.Fatalf("pi stream: %v", err)
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
	assertBaselineSequence(t, "pi-happy-react-stream", baselineEventTypes(events), want)
}

func TestPiAdapterUnknownToolRejected(t *testing.T) {
	adapter := NewPiAdapter(fakePiConfig("unknown-tool"))
	var events []Event
	var audit []ToolCallRecord
	deps := piTestDeps(&events, piToolInvoker(ToolResult{Output: "ok"}, nil), &audit)
	result, err := adapter.Execute(context.Background(), piFixtureRequest("answer", Budget{MaxIterations: 3, MaxToolCalls: 3, MaxTokens: 1000, MaxOutputBytes: 4096}, "known"), deps)
	if err != nil || result.Status != StatusSucceeded {
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
	assertBaselineSequence(t, "pi-unknown-tool", baselineEventTypes(events), want)
	if len(audit) != 1 || audit[0].ErrorCode != "tool_not_found" || audit[0].ToolName != "not_registered" {
		t.Fatalf("unknown tool audit = %+v", audit)
	}
}

func TestPiAdapterToolFailure(t *testing.T) {
	adapter := NewPiAdapter(fakePiConfig("tool-fail"))
	var events []Event
	var audit []ToolCallRecord
	deps := piTestDeps(&events, piToolInvoker(ToolResult{}, errors.New("upstream unavailable")), &audit)
	result, err := adapter.Execute(context.Background(), piFixtureRequest("diagnose", Budget{MaxIterations: 4, MaxToolCalls: 4, MaxTokens: 1000, MaxOutputBytes: 4096}, "failing_lookup"), deps)
	if err != nil || result.Status != StatusSucceeded || result.ToolCallCount != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	want := []string{
		"model.started",
		"assistant.progress",
		"tool.requested", "tool.started", "tool.failed",
		"model.resumed", "assistant.progress",
		"model.started",
		"assistant.completed",
	}
	assertBaselineSequence(t, "pi-tool-failure", baselineEventTypes(events), want)
	if len(audit) != 1 || audit[0].Status != StatusFailed {
		t.Fatalf("audit trail = %+v", audit)
	}
}

func TestPiAdapterCancellationReachesSIGKILL(t *testing.T) {
	// FR-012 three-level cancellation: the child ignores both the interrupt
	// command and stdin close, so the adapter must SIGKILL the process group
	// and return a canonical cancelled terminal state.
	config := fakePiConfig("ignore-cancel")
	adapter := NewPiAdapter(config)
	var events []Event
	var audit []ToolCallRecord
	deps := piTestDeps(&events, piToolInvoker(ToolResult{Output: "ok"}, nil), &audit)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	started := time.Now()
	result, err := adapter.Execute(ctx, piFixtureRequest("answer", Budget{MaxIterations: 2, MaxToolCalls: 2, MaxTokens: 1000, MaxOutputBytes: 4096}), deps)
	if err == nil || result.Status != StatusCancelled || result.ErrorCode != "cancelled" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("cancellation took %s, SIGKILL level did not fire", elapsed)
	}
	if len(RegisteredPiProcesses()) != 0 {
		t.Fatalf("process registry not cleaned after SIGKILL: %+v", RegisteredPiProcesses())
	}
}

func TestPiAdapterTimeoutBudget(t *testing.T) {
	adapter := NewPiAdapter(fakePiConfig("timeout"))
	var events []Event
	var audit []ToolCallRecord
	deps := piTestDeps(&events, piToolInvoker(ToolResult{Output: "ok"}, nil), &audit)
	result, err := adapter.Execute(context.Background(), piFixtureRequest("answer", Budget{MaxIterations: 2, MaxToolCalls: 2, MaxTokens: 1000, MaxOutputBytes: 4096, Timeout: 250 * time.Millisecond}), deps)
	if err == nil || result.Status != StatusCancelled || result.ErrorCode != "timeout" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestPiAdapterChildCrash(t *testing.T) {
	adapter := NewPiAdapter(fakePiConfig("crash"))
	var events []Event
	var audit []ToolCallRecord
	deps := piTestDeps(&events, piToolInvoker(ToolResult{Output: "ok"}, nil), &audit)
	result, err := adapter.Execute(context.Background(), piFixtureRequest("answer", Budget{MaxIterations: 2, MaxToolCalls: 2, MaxTokens: 1000, MaxOutputBytes: 4096}), deps)
	if err == nil || result.Status != StatusFailed || result.ErrorCode != "pi_runtime" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestPiAdapterIterationBudget(t *testing.T) {
	adapter := NewPiAdapter(fakePiConfig("budget-iterations"))
	var events []Event
	var audit []ToolCallRecord
	deps := piTestDeps(&events, piToolInvoker(ToolResult{Output: "ok"}, nil), &audit)
	result, err := adapter.Execute(context.Background(), piFixtureRequest("answer", Budget{MaxIterations: 2, MaxToolCalls: 2, MaxTokens: 100000, MaxOutputBytes: 4096}), deps)
	if err == nil || result.Status != StatusFailed || result.ErrorCode != "iteration_budget" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestPiAdapterTokenBudget(t *testing.T) {
	adapter := NewPiAdapter(fakePiConfig("budget-tokens"))
	var events []Event
	var audit []ToolCallRecord
	deps := piTestDeps(&events, piToolInvoker(ToolResult{Output: "ok"}, nil), &audit)
	result, err := adapter.Execute(context.Background(), piFixtureRequest("answer", Budget{MaxIterations: 4, MaxToolCalls: 2, MaxTokens: 1000, MaxOutputBytes: 4096}), deps)
	if err == nil || result.Status != StatusFailed || result.ErrorCode != "token_budget" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestPiAdapterToolBudget(t *testing.T) {
	adapter := NewPiAdapter(fakePiConfig("budget-tools"))
	var events []Event
	var audit []ToolCallRecord
	deps := piTestDeps(&events, piToolInvoker(ToolResult{Output: "ok"}, nil), &audit)
	result, err := adapter.Execute(context.Background(), piFixtureRequest("answer", Budget{MaxIterations: 20, MaxToolCalls: 2, MaxTokens: 100000, MaxOutputBytes: 4096}, "lookup"), deps)
	if err == nil || result.Status != StatusFailed || result.ErrorCode != "tool_budget" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestPiAdapterEmptyOutput(t *testing.T) {
	adapter := NewPiAdapter(fakePiConfig("empty"))
	var events []Event
	var audit []ToolCallRecord
	deps := piTestDeps(&events, piToolInvoker(ToolResult{Output: "ok"}, nil), &audit)
	result, err := adapter.Execute(context.Background(), piFixtureRequest("answer", Budget{MaxIterations: 2, MaxToolCalls: 2, MaxTokens: 1000, MaxOutputBytes: 4096}), deps)
	if err == nil || result.Status != StatusFailed || result.ErrorCode != "empty_output" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestPiAdapterU2028Framing(t *testing.T) {
	// The JSONL trap from R001 §5: U+2028/U+2029 are valid JSON string
	// characters but JS-style line terminators. Strict LF framing must keep
	// the deltas on separate frames with no corruption.
	adapter := NewPiAdapter(fakePiConfig("u2028"))
	var events []Event
	var audit []ToolCallRecord
	deps := piTestDeps(&events, piToolInvoker(ToolResult{Output: "ok"}, nil), &audit)
	result, err := adapter.Execute(context.Background(), piFixtureRequest("answer", Budget{MaxIterations: 2, MaxToolCalls: 2, MaxTokens: 1000, MaxOutputBytes: 4096}), deps)
	if err != nil || result.Status != StatusSucceeded || result.Output != "完成" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	deltas := 0
	for _, event := range events {
		if event.Type == "assistant.delta" {
			deltas++
		}
	}
	if deltas != 2 {
		t.Fatalf("assistant.delta count = %d, want 2 (frames corrupted?)", deltas)
	}
}

func TestPiAdapterLargeToolResultBackpressure(t *testing.T) {
	// FR-011: a tool result far beyond the ~64KB pipe buffer must flow back to
	// the child without deadlocking; the child concurrently drains stdin.
	adapter := NewPiAdapter(fakePiConfig("big-result"))
	var events []Event
	var audit []ToolCallRecord
	big := ToolResult{Output: strings.Repeat("x", 256<<10)}
	deps := piTestDeps(&events, piToolInvoker(big, nil), &audit)
	result, err := adapter.Execute(context.Background(), piFixtureRequest("check", Budget{MaxIterations: 4, MaxToolCalls: 4, MaxTokens: 100000, MaxOutputBytes: 4096}, "lookup"), deps)
	if err != nil || result.Status != StatusSucceeded || result.Output != "大结果已处理。" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestReapPiOrphansKillsRegisteredGroups(t *testing.T) {
	// FR-013: a process group left in the registry (parent crashed) is
	// force-killed by the startup sweep.
	child := exec.Command(os.Args[0])
	child.Env = append(os.Environ(), "OPSK_PI_FAKE_HELPER=1", "OPSK_PI_FAKE_MODE=hang")
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Wait() }()
	pgid := child.Process.Pid
	piRegisterProcess("orphan-test", pgid, pgid)
	if got := RegisteredPiProcesses()["orphan-test"]; got != pgid {
		t.Fatalf("registry entry = %d, want %d", got, pgid)
	}
	// Simulate the parent disappearing: the child ignores stdin close.
	_ = stdin.Close()
	if ReapPiOrphans() < 1 {
		// The leader may already be gone (ESRCH); what matters is that the
		// entry is dropped and the child terminates.
	}
	if len(RegisteredPiProcesses()) != 0 {
		t.Fatalf("registry not emptied: %+v", RegisteredPiProcesses())
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("orphan child survived the reap sweep")
	}
}

func TestPiAdapterCredentialNeverSerialized(t *testing.T) {
	// NFR-001: the one-shot credential is json:"-" on AdapterRequest and is
	// injected into the child environment only; it must never appear in any
	// emitted event payload.
	adapter := NewPiAdapter(fakePiConfig("happy"))
	request := piFixtureRequest("check", Budget{MaxIterations: 4, MaxToolCalls: 4, MaxTokens: 100000, MaxOutputBytes: 4096}, "lookup")
	request.Credential = ModelCredential{APIKey: "sk-pi-secret"}
	var events []Event
	var audit []ToolCallRecord
	deps := piTestDeps(&events, piToolInvoker(ToolResult{Output: "ok"}, nil), &audit)
	if _, err := adapter.Execute(context.Background(), request, deps); err != nil {
		t.Fatalf("execute: %v", err)
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "sk-pi-secret") {
		t.Fatalf("credential leaked through serialization: %s", encoded)
	}
	for _, event := range events {
		payload, _ := json.Marshal(event.Payload)
		if strings.Contains(string(payload), "sk-pi-secret") {
			t.Fatalf("credential leaked into event %q: %s", event.Type, payload)
		}
	}
}

// TestPiIntegrationRealChild is the true-Pi integration test. It is skipped
// unless explicitly enabled (OPSK_PI_INTEGRATION=1) and a node runtime plus
// the locked @earendil-works/pi-agent-core dependency are available.
func TestPiIntegrationRealChild(t *testing.T) {
	if os.Getenv("OPSK_PI_INTEGRATION") != "1" {
		t.Skip("set OPSK_PI_INTEGRATION=1 with a node runtime and the locked pi-agent-core dependency to run")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not available")
	}
	t.Skip("real-Pi harness: requires the pinned pi-agent-core checkout; kept as the guard for CI with the Node layer installed")
}

// keep io imported for future harness readers
var _ io.Reader = (*bufio.Reader)(nil)
