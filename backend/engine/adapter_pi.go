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
	"time"
)

// This file implements the Pi runtime adapter (I008 C1-2/C1-3, R001
// FR-005/FR-006/FR-007/FR-011/FR-012/FR-013). It drives a per-execution
// `pi --mode rpc` subprocess over a strict-LF JSONL protocol and maps the Pi
// event family onto the frozen unified event catalog. The platform layer keeps
// owning authorization, tool policy, budget verdicts, audit and the execution
// lifecycle: this adapter never emits execution.* events and never calls a
// tool outside deps.Tool (which routes through PolicyGateway).

// PiAdapterID is the registry key of the Pi-backed runtime.
const PiAdapterID = "pi"

// PiAgentCoreVersion is the verified, locked npm package version this adapter
// speaks the protocol of (R001 §2.7 version lock).
const PiAgentCoreVersion = "@earendil-works/pi-agent-core@1.5.2"

// PiEnableEnv is the feature flag that registers the Pi adapter. The Pi
// subprocess layer stays fully disabled unless this flag is set (R001 §9.4).
const PiEnableEnv = "OPSK_ENABLE_PI_RUNTIME"

// PiRuntimeEnabled reports whether the Pi runtime layer should be registered
// for this process.
func PiRuntimeEnabled() bool {
	return strings.TrimSpace(os.Getenv(PiEnableEnv)) == "1"
}

// PiProcessConfig controls how the Pi subprocess is launched and terminated.
// Zero values fall back to the PiProcessDefaults below; tests replace Command
// with a fake child that speaks the same JSONL protocol.
type PiProcessConfig struct {
	// Command is the executable launched per execution.
	Command string
	// Args are the fixed RPC-mode arguments appended before the prompt command.
	Args []string
	// Env carries deployment-level environment variables (never credentials).
	Env []string
	// StartTimeout bounds the ready-handshake wait.
	StartTimeout time.Duration
	// InterruptGrace is the level-1 window after the interrupt command.
	InterruptGrace time.Duration
	// GracePeriod is the level-2 window after stdin is closed (FR-012).
	GracePeriod time.Duration
	// StdinWriteTimeout bounds every command write so a stalled child cannot
	// deadlock the adapter on a full pipe (FR-011).
	StdinWriteTimeout time.Duration
	// MaxOutputLines and MaxOutputBytes bound the event stream drained from
	// the child (FR-011).
	MaxOutputLines int
	MaxOutputBytes int64
}

// PiProcessDefaults returns the production defaults.
func PiProcessDefaults() PiProcessConfig {
	return PiProcessConfig{
		Command:           "pi",
		Args:              []string{"--mode", "rpc", "--no-session"},
		StartTimeout:      30 * time.Second,
		InterruptGrace:    5 * time.Second,
		GracePeriod:       5 * time.Second,
		StdinWriteTimeout: 10 * time.Second,
		MaxOutputLines:    1 << 18,
		MaxOutputBytes:    256 << 20,
	}
}

func (c PiProcessConfig) withDefaults() PiProcessConfig {
	def := PiProcessDefaults()
	if c.Command == "" {
		c.Command = def.Command
	}
	if c.Args == nil {
		c.Args = def.Args
	}
	if c.StartTimeout <= 0 {
		c.StartTimeout = def.StartTimeout
	}
	if c.InterruptGrace <= 0 {
		c.InterruptGrace = def.InterruptGrace
	}
	if c.GracePeriod <= 0 {
		c.GracePeriod = def.GracePeriod
	}
	if c.StdinWriteTimeout <= 0 {
		c.StdinWriteTimeout = def.StdinWriteTimeout
	}
	if c.MaxOutputLines <= 0 {
		c.MaxOutputLines = def.MaxOutputLines
	}
	if c.MaxOutputBytes <= 0 {
		c.MaxOutputBytes = def.MaxOutputBytes
	}
	return c
}

// PiAdapter adapts the Pi agent runtime (per-execution subprocess + JSONL
// RPC) to the framework-neutral EngineAdapter contract.
type PiAdapter struct {
	config PiProcessConfig
}

// NewPiAdapter creates the Pi runtime adapter.
func NewPiAdapter(config PiProcessConfig) *PiAdapter {
	return &PiAdapter{config: config.withDefaults()}
}

// ID implements EngineAdapter.
func (a *PiAdapter) ID() string { return PiAdapterID }

// Version implements EngineAdapter. It cites the locked pi-agent-core
// package the protocol was verified against.
func (a *PiAdapter) Version() string { return PiAgentCoreVersion }

// Experimental reports that this runtime is registered behind the feature
// flag and must never become a scenario default (R001 §9.2).
func (a *PiAdapter) Experimental() bool { return true }

// Capabilities implements EngineAdapter. Structured output is validated on
// the Go side of the adapter boundary (FR-009 degradation path); context
// compaction is intentionally not declared because the subprocess does not
// own context window management.
func (a *PiAdapter) Capabilities() CapabilitySet {
	return CapabilitySet{
		AdapterCapabilityAgentLoop,
		AdapterCapabilityToolCalling,
		AdapterCapabilityStreaming,
		AdapterCapabilityStructuredOutput,
	}
}

// Execute implements EngineAdapter.
func (a *PiAdapter) Execute(ctx context.Context, request AdapterRequest, deps AdapterDeps) (AdapterResult, error) {
	return a.run(ctx, request, deps)
}

// Stream implements EngineAdapter. The JSONL event stream is mapped onto
// unified events through deps.Emit while the loop runs.
func (a *PiAdapter) Stream(ctx context.Context, request AdapterRequest, deps AdapterDeps) (AdapterResult, error) {
	request.Stream = true
	return a.run(ctx, request, deps)
}

// ---------------------------------------------------------------------------
// JSONL protocol shapes
// ---------------------------------------------------------------------------

type piToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type piPromptParams struct {
	Prompt   string            `json:"prompt"`
	System   string            `json:"system,omitempty"`
	Tools    []piToolSpec      `json:"tools,omitempty"`
	Model    piModelSpec       `json:"model"`
	Budget   map[string]any    `json:"budget,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// piModelSpec is the unified model description injected into the child as a
// custom provider configuration (ruling §10-1: credentials travel one-way,
// through the environment only, never inside the command payload).
type piModelSpec struct {
	ProviderID          string        `json:"provider_id"`
	ModelName           string        `json:"model_name"`
	Protocol            ModelProtocol `json:"protocol"`
	ContextWindowTokens int           `json:"context_window_tokens,omitempty"`
	MaxOutputTokens     int           `json:"max_output_tokens,omitempty"`
	Temperature         float64       `json:"temperature,omitempty"`
}

type piCommand struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type piUsage struct {
	InputTokens  int64 `json:"input_tokens,omitempty"`
	OutputTokens int64 `json:"output_tokens,omitempty"`
	TotalTokens  int64 `json:"total_tokens,omitempty"`
}

type piEvent struct {
	Type        string          `json:"type"`
	ID          string          `json:"id,omitempty"`
	PID         int             `json:"pid,omitempty"`
	Turn        int             `json:"turn,omitempty"`
	Role        string          `json:"role,omitempty"`
	MessageType string          `json:"message_type,omitempty"`
	Delta       string          `json:"delta,omitempty"`
	Thinking    bool            `json:"thinking,omitempty"`
	CallID      string          `json:"call_id,omitempty"`
	Tool        string          `json:"tool,omitempty"`
	Args        map[string]any  `json:"args,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
	Usage       *piUsage        `json:"usage,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	Text        string          `json:"text,omitempty"`
}

// ---------------------------------------------------------------------------
// Orphan process registry (FR-013)
// ---------------------------------------------------------------------------

type piProcessEntry struct {
	pgid      int
	pid       int
	startedAt time.Time
	execution string
}

var piProcessRegistry = struct {
	sync.Mutex
	entries map[string]piProcessEntry
}{entries: make(map[string]piProcessEntry)}

func piRegisterProcess(executionID string, pid, pgid int) {
	if executionID == "" || pgid <= 0 {
		return
	}
	piProcessRegistry.Lock()
	defer piProcessRegistry.Unlock()
	piProcessRegistry.entries[executionID] = piProcessEntry{pgid: pgid, pid: pid, startedAt: time.Now().UTC(), execution: executionID}
}

func piUnregisterProcess(executionID string) {
	piProcessRegistry.Lock()
	defer piProcessRegistry.Unlock()
	delete(piProcessRegistry.entries, executionID)
}

// ReapPiOrphans force-kills every still-registered Pi process group. The API
// and Worker call it at startup so a crashed parent never leaves a child
// holding credential environment variables (FR-013). Process groups whose
// leader already exited return ESRCH, which is treated as clean.
func ReapPiOrphans() int {
	piProcessRegistry.Lock()
	entries := make([]piProcessEntry, 0, len(piProcessRegistry.entries))
	for _, entry := range piProcessRegistry.entries {
		entries = append(entries, entry)
	}
	piProcessRegistry.entries = make(map[string]piProcessEntry)
	piProcessRegistry.Unlock()
	reaped := 0
	for _, entry := range entries {
		if err := syscall.Kill(-entry.pgid, syscall.SIGKILL); err == nil {
			reaped++
		}
	}
	return reaped
}

// RegisteredPiProcesses reports the live execution->process-group table; it
// exists for diagnostics and tests.
func RegisteredPiProcesses() map[string]int {
	piProcessRegistry.Lock()
	defer piProcessRegistry.Unlock()
	out := make(map[string]int, len(piProcessRegistry.entries))
	for id, entry := range piProcessRegistry.entries {
		out[id] = entry.pgid
	}
	return out
}

// ---------------------------------------------------------------------------
// Execution
// ---------------------------------------------------------------------------

func (a *PiAdapter) run(ctx context.Context, request AdapterRequest, deps AdapterDeps) (AdapterResult, error) {
	if a == nil {
		return AdapterResult{}, ErrRunnerUnavailable
	}
	if err := ValidateAdapterRequest(request, a); err != nil {
		return AdapterResult{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	config := a.config.withDefaults()
	budget := request.Budget
	if budget.MaxIterations <= 0 {
		budget.MaxIterations = DefaultBudget().MaxIterations
	}
	if budget.MaxToolCalls <= 0 {
		budget.MaxToolCalls = DefaultBudget().MaxToolCalls
	}
	if budget.MaxTokens <= 0 {
		budget.MaxTokens = DefaultBudget().MaxTokens
	}
	if budget.MaxOutputBytes <= 0 {
		budget.MaxOutputBytes = DefaultBudget().MaxOutputBytes
	}
	if request.ExecutionID == "" {
		request.ExecutionID = fmt.Sprintf("pi-%d", time.Now().UnixNano())
	}
	timeoutCtx := ctx
	var timeoutCancel context.CancelFunc
	if budget.Timeout > 0 && !budget.NoExecutionTimeout {
		timeoutCtx, timeoutCancel = context.WithTimeout(ctx, budget.Timeout)
		defer timeoutCancel()
	}
	runCtx, cancel := context.WithCancel(timeoutCtx)
	defer cancel()

	state := &piRunState{
		request: request, budget: budget, deps: deps, config: config,
		tools: make(map[string]ToolDeclaration, len(request.Tools)),
	}
	for _, declaration := range request.Tools {
		if name := strings.TrimSpace(declaration.Name); name != "" {
			state.tools[name] = declaration
		}
	}

	cmd := exec.Command(config.Command, config.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = piChildEnv(config, request)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return AdapterResult{}, fmt.Errorf("pi stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return AdapterResult{}, fmt.Errorf("pi stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return AdapterResult{}, fmt.Errorf("start pi runtime: %w", err)
	}
	pgid := cmd.Process.Pid
	piRegisterProcess(request.ExecutionID, cmd.Process.Pid, pgid)
	defer piUnregisterProcess(request.ExecutionID)
	state.stdin = stdin
	state.writer = bufio.NewWriter(stdin)

	// The three-level cancellation watcher (FR-012): context cancel maps to
	// the RPC interrupt command, then stdin close, then a process-group
	// SIGKILL. The read loop exits through done.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-runCtx.Done():
		case <-done:
			return
		}
		state.noteCancelReason(runCtx.Err())
		// Level 1: RPC-level interrupt.
		_ = state.writeCommand(piCommand{ID: "pi-interrupt", Method: "interrupt"})
		select {
		case <-time.After(config.InterruptGrace):
		case <-done:
			return
		}
		// Level 2: close stdin for a graceful exit.
		_ = stdin.Close()
		select {
		case <-time.After(config.GracePeriod):
		case <-done:
			return
		}
		// Level 3: SIGKILL the whole process group (FR-011 setpgid).
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}()

	result, runErr := state.consume(stdout)
	// Ensure the child is gone before returning, mirroring the same three
	// levels in a bounded way for the success path.
	_ = stdin.Close()
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	select {
	case <-waitDone:
	case <-time.After(config.GracePeriod):
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		<-waitDone
	}
	return result, runErr
}

// piChildEnv builds the child environment. Credentials travel exactly once,
// one-way, through the environment; they never appear in commands, events,
// logs or audit records (NFR-001).
func piChildEnv(config PiProcessConfig, request AdapterRequest) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, config.Env...)
	env = append(env,
		"OPSK_PI_EXECUTION_ID="+request.ExecutionID,
		"OPSK_PI_PROVIDER_ID="+request.Model.ProviderID,
		"OPSK_PI_MODEL_NAME="+request.Model.ModelName,
		"OPSK_PI_MODEL_PROTOCOL="+string(request.Model.Protocol),
	)
	if !request.Credential.Empty() {
		if request.Credential.APIKey != "" {
			env = append(env, "OPSK_PI_MODEL_API_KEY="+request.Credential.APIKey)
		}
		if request.Credential.BaseURL != "" {
			env = append(env, "OPSK_PI_MODEL_BASE_URL="+request.Credential.BaseURL)
		}
	}
	return env
}

type piRunState struct {
	request   AdapterRequest
	budget    Budget
	deps      AdapterDeps
	config    PiProcessConfig
	tools     map[string]ToolDeclaration
	stdin     io.WriteCloser
	writer    *bufio.Writer
	writeMu   sync.Mutex
	usage     piUsage
	output    strings.Builder
	toolCalls int
	emitMu    sync.Mutex

	iterations       int
	turnHasTools     bool
	turnProgressSent bool
	turnDeltas       strings.Builder
	settled          bool
	cancelReason     error
}

func (s *piRunState) noteCancelReason(err error) {
	s.emitMu.Lock()
	if s.cancelReason == nil {
		s.cancelReason = err
	}
	s.emitMu.Unlock()
}

func (s *piRunState) emit(event Event) {
	if s.deps.Emit == nil {
		return
	}
	s.emitMu.Lock()
	defer s.emitMu.Unlock()
	event.ExecutionID = s.request.ExecutionID
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}
	_ = s.deps.Emit(event)
}

func (s *piRunState) progress(text string, kind string, iteration int) {
	s.emit(Event{Type: "assistant.progress", Status: StatusRunning, Payload: map[string]any{
		"text": text, "iteration": iteration, "kind": kind,
	}})
}

// writeCommand writes one JSONL command with strict LF framing. encoding/json
// escapes U+2028/U+2029 inside strings, so a delta can never smuggle a frame
// boundary. Every write is bounded by StdinWriteTimeout (FR-011).
func (s *piRunState) writeCommand(command piCommand) error {
	encoded, err := json.Marshal(command)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.writer == nil {
		return errors.New("pi stdin is closed")
	}
	written := make(chan error, 1)
	go func() {
		if _, err := s.writer.Write(encoded); err != nil {
			written <- err
			return
		}
		written <- s.writer.Flush()
	}()
	select {
	case err := <-written:
		return err
	case <-time.After(s.config.StdinWriteTimeout):
		return errors.New("pi stdin write timed out")
	}
}

func (s *piRunState) fail(code, message string) (AdapterResult, error) {
	s.emit(Event{Type: "phase.changed", Status: StatusFailed, Payload: map[string]any{
		"phase": "failed", "detail": message, "reason": code, "iteration": s.iterations,
	}})
	return s.result(StatusFailed, code, message), errors.New(message)
}

func (s *piRunState) result(status Status, code, message string) AdapterResult {
	output := scrubVisibleText(strings.TrimSpace(s.output.String()))
	return AdapterResult{
		ExecutionID: s.request.ExecutionID, Status: status, Output: output,
		PromptTokens: s.usage.InputTokens, CompletionTokens: s.usage.OutputTokens,
		TotalTokens: s.totalTokens(), ToolCallCount: s.toolCalls,
		ErrorCode: code, ErrorMessage: message,
	}
}

func (s *piRunState) totalTokens() int64 {
	if s.usage.TotalTokens > 0 {
		return s.usage.TotalTokens
	}
	return s.usage.InputTokens + s.usage.OutputTokens
}

// consume drains the JSONL event stream (its own goroutine path through the
// read loop) and drives the unified event mapping until the child settles,
// crashes or a budget boundary stops the loop.
func (s *piRunState) consume(stdout io.Reader) (AdapterResult, error) {
	reader := bufio.NewReaderSize(stdout, 64<<10)
	// Level-0 readiness handshake: the child must report its own PID before
	// any prompt is accepted (FR-013).
	ready := make(chan error, 1)
	go func() {
		event, err := readPiEvent(reader)
		if err != nil {
			ready <- err
			return
		}
		if event.Type != "ready" {
			ready <- fmt.Errorf("pi handshake expected ready, got %q", event.Type)
			return
		}
		if event.PID <= 0 {
			ready <- errors.New("pi handshake did not report a pid")
			return
		}
		ready <- nil
	}()
	select {
	case err := <-ready:
		if err != nil {
			return s.fail("pi_runtime", "Pi 运行时握手失败："+publicError(err))
		}
	case <-time.After(s.config.StartTimeout):
		return s.fail("pi_runtime", "Pi 运行时启动超时")
	}
	prompt := strings.TrimSpace(s.request.Prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(s.request.Task)
	}
	params, err := json.Marshal(piPromptParams{
		Prompt: prompt, System: strings.TrimSpace(s.request.Instruction),
		Tools: piToolSpecs(s.request), Model: piModelSpec{
			ProviderID: s.request.Model.ProviderID, ModelName: s.request.Model.ModelName,
			Protocol: s.request.Model.Protocol, ContextWindowTokens: s.request.Model.ContextWindowTokens,
			MaxOutputTokens: s.request.Model.MaxOutputTokens, Temperature: s.request.Model.Temperature,
		},
		Budget: map[string]any{
			"max_iterations": s.budget.MaxIterations,
			"max_tool_calls": s.budget.MaxToolCalls,
			"max_tokens":     s.budget.MaxTokens,
		},
		Metadata: map[string]string{"execution_id": s.request.ExecutionID, "runtime": PiAdapterID},
	})
	if err != nil {
		return AdapterResult{}, err
	}
	if err := s.writeCommand(piCommand{ID: "pi-prompt", Method: "prompt", Params: params}); err != nil {
		return s.fail("pi_runtime", "Pi 运行时指令下发失败："+publicError(err))
	}

	var lines int
	var bytes int64
	for {
		if s.cancelReason != nil {
			code := "cancelled"
			if errors.Is(s.cancelReason, context.DeadlineExceeded) {
				code = "timeout"
			}
			result := s.result(StatusCancelled, code, publicError(s.cancelReason))
			if s.deps.Cancel != nil {
				s.deps.Cancel(code)
			}
			return result, s.cancelReason
		}
		event, err := readPiEvent(reader)
		if err != nil {
			if s.settled {
				return s.settledResult()
			}
			if s.cancelReason != nil {
				code := "cancelled"
				if errors.Is(s.cancelReason, context.DeadlineExceeded) {
					code = "timeout"
				}
				return s.result(StatusCancelled, code, publicError(s.cancelReason)), s.cancelReason
			}
			return s.fail("pi_runtime", "Pi 运行时进程异常退出："+publicError(err))
		}
		lines++
		bytes += int64(len(event.Delta)) + 64
		if lines > s.config.MaxOutputLines || bytes > s.config.MaxOutputBytes {
			return s.fail("pi_runtime", "Pi 运行时输出超出资源上限")
		}
		if result, done, err := s.handleEvent(event); done {
			return result, err
		}
	}
}

func readPiEvent(reader *bufio.Reader) (piEvent, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return piEvent{}, err
	}
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return readPiEvent(reader)
	}
	var event piEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return piEvent{}, fmt.Errorf("malformed pi event frame: %w", err)
	}
	return event, nil
}

func piToolSpecs(request AdapterRequest) []piToolSpec {
	specs := make([]piToolSpec, 0, len(request.Tools))
	for _, declaration := range request.Tools {
		if name := strings.TrimSpace(declaration.Name); name != "" {
			specs = append(specs, piToolSpec{Name: name, Description: declaration.Description, InputSchema: declaration.InputSchema})
		}
	}
	return specs
}

// handleEvent maps one Pi protocol event onto the unified catalog. It returns
// done=true when the execution reached a terminal state.
func (s *piRunState) handleEvent(event piEvent) (AdapterResult, bool, error) {
	switch event.Type {
	case "agent_start", "message_start":
		return AdapterResult{}, false, nil
	case "turn_start":
		s.iterations++
		if s.iterations > s.budget.MaxIterations {
			result, err := s.fail("iteration_budget", "iteration budget exceeded")
			return result, true, err
		}
		s.turnHasTools = false
		s.turnProgressSent = false
		s.turnDeltas.Reset()
		s.emit(Event{Type: "model.started", Status: StatusRunning, Payload: map[string]any{
			"iteration": s.iterations, "detail": "正在分析当前目标并评估下一步",
		}})
		return AdapterResult{}, false, nil
	case "message_update":
		// Ruling §10-4: thought deltas that cannot be proven non-thinking are
		// dropped; only explicit text deltas reach assistant.delta.
		if event.Thinking || event.MessageType != "text" || event.Delta == "" {
			return AdapterResult{}, false, nil
		}
		if text := safeStreamingDelta(event.Delta, 0); text != "" {
			s.turnDeltas.WriteString(text)
			s.emit(Event{Type: "assistant.delta", Status: StatusRunning, Payload: map[string]any{
				"text": text, "iteration": s.iterations,
			}})
		}
		return AdapterResult{}, false, nil
	case "tool_execution_start":
		result, done, err := s.handleToolCall(event)
		return result, done, err
	case "tool_execution_end":
		// Tool results are owned by the forwarding stub: the platform already
		// emitted tool.completed/tool.failed when the gateway returned. A
		// late or duplicated end event is deduplicated by call ID and never
		// overwrites the platform terminal state (FR-012 idempotent merge).
		return AdapterResult{}, false, nil
	case "message_end":
		if event.Usage != nil {
			s.usage.InputTokens += event.Usage.InputTokens
			s.usage.OutputTokens += event.Usage.OutputTokens
			s.usage.TotalTokens += event.Usage.TotalTokens
			if s.totalTokens() > s.budget.MaxTokens {
				detail := fmt.Sprintf("模型累计 Token 预算已用尽（已使用 %d / %d）", s.totalTokens(), s.budget.MaxTokens)
				s.emit(Event{Type: "phase.changed", Status: StatusFailed, Payload: map[string]any{
					"phase": "budget_exceeded", "detail": detail, "reason": "token_budget",
					"iteration": s.iterations, "used_tokens": s.totalTokens(), "max_tokens": s.budget.MaxTokens,
				}})
				result := s.result(StatusFailed, "token_budget", detail)
				return result, true, errors.New(detail)
			}
		}
		return AdapterResult{}, false, nil
	case "turn_end":
		if s.turnHasTools {
			observations := []map[string]any{{"outcome": "success"}}
			emitModelResumed(func(event Event) error { s.emit(event); return nil }, s.request.ExecutionID, s.iterations, observations)
			s.progress("正在分析当前目标并评估下一步", "analysis", s.iterations)
		} else if text := strings.TrimSpace(s.turnDeltas.String()); text != "" {
			s.output.WriteString(text)
		}
		return AdapterResult{}, false, nil
	case "agent_settled", "agent_end":
		s.settled = true
		if text := strings.TrimSpace(event.Text); text != "" {
			s.output.Reset()
			s.output.WriteString(text)
		}
		result, err := s.settledResult()
		return result, true, err
	default:
		// Unknown Pi event families are ignored rather than rejected: the
		// unified catalog stays authoritative and the loop stays forward
		// compatible with locked upstream versions.
		return AdapterResult{}, false, nil
	}
}

// handleToolCall implements the FR-006 forwarding stub: the Pi-side
// tool_execution_start maps to tool.requested, the deterministic tool.started
// synthetic point is the moment the gateway-authorized invocation begins, and
// the gateway outcome closes the lifecycle with tool.completed/tool.failed.
func (s *piRunState) handleToolCall(event piEvent) (AdapterResult, bool, error) {
	toolName := strings.TrimSpace(event.Tool)
	callID := strings.TrimSpace(event.CallID)
	s.toolCalls++
	s.turnHasTools = true
	if !s.turnProgressSent {
		s.turnProgressSent = true
		decision := safeProgressText(strings.TrimSpace(s.turnDeltas.String()), 2000)
		if decision == "" {
			decision = toolDecisionProgressText([]string{toolName})
		}
		s.progress(decision, "tool_decision", s.iterations)
	}
	call := ToolCall{
		ExecutionID: s.request.ExecutionID, Sequence: s.toolCalls,
		ActorID: s.request.ActorID, ScopeID: s.request.ScopeID,
		ProviderID: s.request.ProviderID, ModelName: s.request.ModelName,
		Name: toolName, ModelToolName: toolName, Arguments: event.Args,
		Iteration: s.iterations, CallID: callID,
	}
	s.emit(Event{Type: "tool.requested", Status: StatusRunning, Payload: toolEventPayload(call, toolName, map[string]any{
		"arguments": redactValue(event.Args),
	})})
	if s.toolCalls > s.budget.MaxToolCalls {
		s.recordAudit(call, ToolCallRecord{
			ExecutionID: call.ExecutionID, Sequence: call.Sequence,
			ProviderID: call.ProviderID, ModelName: call.ModelName,
			ToolName: toolName, Arguments: call.Arguments,
			Status: StatusFailed, ErrorCode: "tool_budget", Error: "tool call budget exceeded",
			StartedAt: time.Now().UTC(), CompletedAt: time.Now().UTC(),
		})
		s.emit(Event{Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, toolName, map[string]any{
			"error": "tool call budget exceeded", "error_code": "tool_budget", "duration_ms": int64(0),
		})})
		s.emit(Event{Type: "phase.changed", Status: StatusFailed, Payload: map[string]any{
			"phase": "budget_exceeded", "detail": "工具调用预算已用尽", "reason": "tool_budget",
			"iteration": s.iterations, "tool_call_count": s.toolCalls,
		}})
		result := s.result(StatusFailed, "tool_budget", "tool call budget exceeded")
		return result, true, errors.New("tool call budget exceeded")
	}
	declaration, known := s.tools[toolName]
	if !known || s.request.RestrictTools && !agentToolAllowed(nil, s.request.AllowedTools, true, toolName) {
		// Unknown or unregistered tools never reach the gateway; the attempt
		// is audited and answered with a structured error observation so the
		// model can correct (acceptance criterion 3).
		s.recordAudit(call, ToolCallRecord{
			ExecutionID: call.ExecutionID, Sequence: call.Sequence,
			ProviderID: call.ProviderID, ModelName: call.ModelName,
			ToolName: toolName, Arguments: call.Arguments,
			Status: StatusFailed, ErrorCode: "tool_not_found", Error: "tool not registered",
			StartedAt: time.Now().UTC(), CompletedAt: time.Now().UTC(),
		})
		s.emit(Event{Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, toolName, map[string]any{
			"error": "tool not registered", "error_code": "tool_not_found", "duration_ms": int64(0),
		})})
		_ = s.writeCommand(piCommand{ID: "pi-tool-" + callID, Method: "tool_result", Params: mustMarshalJSON(map[string]any{
			"call_id": callID, "error": "tool not registered", "error_code": "tool_not_found",
		})})
		return AdapterResult{}, false, nil
	}
	resourceID := "adapter:" + declaration.Name
	call.ResourceID = resourceID
	// Deterministic tool.started synthetic point: the call is authorized for
	// gateway dispatch and is about to execute.
	s.emit(Event{Type: "tool.started", Status: StatusRunning, Payload: toolEventPayload(call, toolName, map[string]any{
		"arguments": redactValue(event.Args),
	})})
	started := time.Now()
	if s.deps.Tool == nil {
		s.emit(Event{Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, toolName, map[string]any{
			"error": "tool gateway unavailable", "error_code": "tool_gateway", "duration_ms": time.Since(started).Milliseconds(),
		})})
		return AdapterResult{}, false, nil
	}
	toolResult, invokeErr := s.deps.Tool.InvokeTool(context.Background(), call)
	duration := time.Since(started).Milliseconds()
	if invokeErr != nil {
		message := publicError(invokeErr)
		if message == "" {
			message = "工具调用失败"
		}
		code := classifyToolResponseError(message)
		s.recordAudit(call, ToolCallRecord{
			ExecutionID: call.ExecutionID, Sequence: call.Sequence,
			ProviderID: call.ProviderID, ModelName: call.ModelName,
			ResourceID: resourceID, ToolName: declaration.Name, Arguments: call.Arguments,
			Status: StatusFailed, ErrorCode: code, Error: message,
			StartedAt: started.UTC(), CompletedAt: time.Now().UTC(),
		})
		s.emit(Event{Type: "tool.failed", Status: StatusFailed, Payload: toolEventPayload(call, toolName, map[string]any{
			"error": message, "error_code": code, "duration_ms": duration,
		})})
		_ = s.writeCommand(piCommand{ID: "pi-tool-" + callID, Method: "tool_result", Params: mustMarshalJSON(map[string]any{
			"call_id": callID, "error": message, "error_code": code,
		})})
		return AdapterResult{}, false, nil
	}
	observation := summarizeObservation(toolResult.Output)
	s.recordAudit(call, ToolCallRecord{
		ExecutionID: call.ExecutionID, Sequence: call.Sequence,
		ProviderID: call.ProviderID, ModelName: call.ModelName,
		ResourceID: resourceID, ToolName: declaration.Name, Arguments: call.Arguments,
		Status:    StatusSucceeded,
		StartedAt: started.UTC(), CompletedAt: time.Now().UTC(),
	})
	s.emit(Event{Type: "tool.completed", Status: StatusSucceeded, Payload: toolEventPayload(call, toolName, map[string]any{
		"output": observation, "duration_ms": duration,
	})})
	_ = s.writeCommand(piCommand{ID: "pi-tool-" + callID, Method: "tool_result", Params: mustMarshalJSON(map[string]any{
		"call_id": callID, "output": map[string]any{"output": observation, "untrusted": true, "engine_observation": true},
	})})
	return AdapterResult{}, false, nil
}

func (s *piRunState) recordAudit(call ToolCall, record ToolCallRecord) {
	if s.deps.Audit == nil {
		return
	}
	_ = s.deps.Audit(context.Background(), record)
}

// settledResult applies the same final-output contract as the platform:
// credential scrubbing, output budget, OutputSchema validation (Go-side
// degradation path, FR-009) and assistant.completed.
func (s *piRunState) settledResult() (AdapterResult, error) {
	text := scrubVisibleText(strings.TrimSpace(s.output.String()))
	if text == "" {
		s.emit(Event{Type: "phase.changed", Status: StatusFailed, Payload: map[string]any{
			"phase": "failed", "detail": "模型未返回可展示的最终回答", "reason": "empty_output", "iteration": s.iterations,
		}})
		message := "模型未返回可展示的最终回答，请重试。"
		return s.result(StatusFailed, "empty_output", message), errors.New(message)
	}
	if len([]byte(text)) > s.budget.MaxOutputBytes {
		s.emit(Event{Type: "phase.changed", Status: StatusFailed, Payload: map[string]any{
			"phase": "budget_exceeded", "detail": "最终输出大小超过预算", "reason": "output_budget", "iteration": s.iterations,
		}})
		return s.result(StatusFailed, "output_budget", "output budget exceeded"), errors.New("output budget exceeded")
	}
	if len(s.request.OutputSchema) > 0 {
		if err := validateJSON(s.request.OutputSchema, []byte(text)); err != nil {
			return s.result(StatusFailed, "agent_output_schema", publicError(err)), err
		}
	}
	s.emit(Event{Type: "assistant.completed", Status: StatusRunning, Payload: map[string]any{
		"text": scrubVisibleText(text), "iteration": s.iterations,
	}})
	return AdapterResult{
		ExecutionID: s.request.ExecutionID, Status: StatusSucceeded, Output: text,
		PromptTokens: s.usage.InputTokens, CompletionTokens: s.usage.OutputTokens,
		TotalTokens: s.totalTokens(), ToolCallCount: s.toolCalls,
	}, nil
}

func mustMarshalJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}
