package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// This file freezes the framework-neutral Runtime Adapter contract (I008 A2-1,
// R001 FR-001). It is a pure declaration: nothing here changes the current ADK
// execution path. The platform layer stays responsible for authorization,
// context resolution, tool policy, budget accounting, audit and execution
// state; an adapter only drives one agent loop inside an already authorized
// snapshot.
//
// Red lines enforced by the shape of this contract:
//   - An adapter never receives raw provider credentials, unauthorized resource
//     IDs or anything that could widen the execution scope.
//   - An adapter never owns the tool gateway. It may only invoke tools through
//     the supplied ToolInvoker, which routes back to PolicyGateway.
//   - An adapter may not emit execution.* terminal events. The platform owns
//     the execution lifecycle.

// Adapter capability keys are the unified, framework-neutral capability
// vocabulary. They are the same keys the model capability matrix already uses
// (see missingCapabilities), so an adapter capability set can be compared
// against a request requirement without translation.
const (
	// AdapterCapabilityAgentLoop means the runtime drives a multi-turn
	// reasoning loop by itself.
	AdapterCapabilityAgentLoop = "agent_loop"
	// AdapterCapabilityToolCalling means the runtime can request platform tools.
	AdapterCapabilityToolCalling = "tool_calling"
	// AdapterCapabilityStreaming means the runtime can emit incremental
	// assistant deltas.
	AdapterCapabilityStreaming = "stream"
	// AdapterCapabilityStructuredOutput means the runtime can be asked for
	// schema-constrained output.
	AdapterCapabilityStructuredOutput = "structured_output"
	// AdapterCapabilityContextCompaction means the runtime can drop or compact
	// older turns itself.
	AdapterCapabilityContextCompaction = "context_compaction"
)

// CapabilitySet is the unified capability description of a runtime adapter.
// It is intentionally a plain slice of stable keys so it can be serialized,
// compared and asserted without pulling in a framework specific type.
type CapabilitySet []string

// Has reports whether the capability set contains the given key.
func (c CapabilitySet) Has(capability string) bool {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return false
	}
	return slices.Contains(c, capability)
}

// Missing returns the required capabilities that the set does not provide.
func (c CapabilitySet) Missing(required []string) []string {
	missing := make([]string, 0)
	for _, capability := range required {
		capability = strings.TrimSpace(capability)
		if capability == "" || c.Has(capability) || slices.Contains(missing, capability) {
			continue
		}
		missing = append(missing, capability)
	}
	return missing
}

// ModelProtocol identifies the wire protocol a provider model speaks. It lets a
// runtime adapter build its own model client without depending on another
// framework's model handle.
type ModelProtocol string

const (
	ModelProtocolOpenAICompatible ModelProtocol = "openai_compatible"
	ModelProtocolAnthropic        ModelProtocol = "anthropic"
	ModelProtocolGemini           ModelProtocol = "gemini"
	ModelProtocolUnknown          ModelProtocol = "unknown"
)

// ModelDescription is the framework-neutral description of the selected model.
// Provider resolution, credential loading and capability checks stay in the
// platform layer; each adapter builds its own client from this description.
// Credentials are never part of this structure: an adapter receives them as a
// one-shot in-memory value through ModelCredential.
type ModelDescription struct {
	ProviderID          string        `json:"provider_id,omitempty"`
	ModelName           string        `json:"model_name,omitempty"`
	Protocol            ModelProtocol `json:"protocol,omitempty"`
	Capabilities        []string      `json:"capabilities,omitempty"`
	ContextWindowTokens int           `json:"context_window_tokens,omitempty"`
	MaxOutputTokens     int           `json:"max_output_tokens,omitempty"`
	Temperature         float64       `json:"temperature,omitempty"`
}

// ModelCredential carries a provider credential for exactly one execution. It
// is intentionally not serializable and must never be persisted by an adapter.
type ModelCredential struct {
	// APIKey is the raw provider secret. It lives in memory only.
	APIKey string `json:"-"`
	// BaseURL overrides the provider endpoint when the deployment requires it.
	BaseURL string `json:"-"`
	// Headers carries additional provider headers, for example a routed
	// provider key.
	Headers map[string]string `json:"-"`
}

// Empty reports whether the credential carries no usable secret.
func (c ModelCredential) Empty() bool {
	return strings.TrimSpace(c.APIKey) == "" && strings.TrimSpace(c.BaseURL) == "" && len(c.Headers) == 0
}

// AdapterRequest is the immutable, already authorized snapshot handed to an
// adapter. Everything in it has been resolved and authorized by the platform
// layer before the adapter is invoked; the adapter must not attempt to widen
// it. The platform re-validates the contract before every call.
type AdapterRequest struct {
	ExecutionID string         `json:"execution_id,omitempty"`
	ActorID     string         `json:"actor_id,omitempty"`
	ScopeID     string         `json:"scope_id"`
	Purpose     Purpose        `json:"purpose,omitempty"`
	Profile     Profile        `json:"profile,omitempty"`
	ProviderID  string         `json:"provider_id,omitempty"`
	ModelName   string         `json:"model_name,omitempty"`
	Task        string         `json:"task,omitempty"`
	Instruction string         `json:"instruction,omitempty"`
	Messages    []Message      `json:"messages,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
	// Prompt is the resolved, sanitized prompt payload. The adapter sends it
	// as the first user turn.
	Prompt       string          `json:"prompt,omitempty"`
	InputSchema  json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`
	// Tools lists the tools the model may call, already filtered by the
	// platform allow-list, persona and resource policy.
	Tools []ToolDeclaration `json:"tools,omitempty"`
	// AllowedTools restricts the model to the listed canonical tool names.
	AllowedTools  []string     `json:"allowed_tools,omitempty"`
	RestrictTools bool         `json:"restrict_tools,omitempty"`
	Requirements  Requirements `json:"requirements,omitempty"`
	Budget        Budget       `json:"budget,omitempty"`
	Stream        bool         `json:"stream,omitempty"`
	// Model describes the selected provider model in framework-neutral terms.
	Model ModelDescription `json:"model,omitempty"`
}

// Validate checks the invariants an adapter request must satisfy before an
// adapter is allowed to run. It is intentionally strict about scope and
// prompt presence and deliberately silent about provider internals.
func (r AdapterRequest) Validate() error {
	if strings.TrimSpace(r.ScopeID) == "" {
		return &ValidationError{Message: "scope_id is required"}
	}
	if strings.TrimSpace(r.Prompt) == "" && strings.TrimSpace(r.Task) == "" && len(r.Messages) == 0 {
		return &ValidationError{Message: "prompt, task or messages is required"}
	}
	if r.Budget.MaxIterations < 0 || r.Budget.MaxToolCalls < 0 || r.Budget.MaxTokens < 0 {
		return &ValidationError{Message: "budget must not be negative"}
	}
	return nil
}

// ToolInvoker is the only path an adapter may use to execute a platform tool.
// The platform implementation routes the call through PolicyGateway, so
// authorization, audit and the tool.* event lifecycle stay in the platform
// layer and are identical for every runtime.
type ToolInvoker interface {
	// InvokeTool executes one authorized tool call. The returned ToolResult is
	// the same shape the platform hands to the model today, so an adapter must
	// treat it as untrusted content and bound it before use.
	InvokeTool(ctx context.Context, call ToolCall) (ToolResult, error)
}

// ToolInvokerFunc adapts a function to the ToolInvoker interface.
type ToolInvokerFunc func(ctx context.Context, call ToolCall) (ToolResult, error)

// InvokeTool implements ToolInvoker.
func (f ToolInvokerFunc) InvokeTool(ctx context.Context, call ToolCall) (ToolResult, error) {
	if f == nil {
		return ToolResult{}, ErrToolInvalid
	}
	return f(ctx, call)
}

// AdapterAuditFunc records one tool call in the platform audit trail. It
// mirrors PolicyGateway's audit sink so an adapter can record calls that never
// reached the gateway (for example a model requested an unregistered tool).
type AdapterAuditFunc func(ctx context.Context, record ToolCallRecord) error

// AdapterDeps carries the platform-owned callbacks an adapter may use. It
// deliberately contains no credential store, no registry and no state
// mutators: an adapter can only ask the platform to invoke a tool, emit an
// event or report an audit record.
type AdapterDeps struct {
	// Tool invokes an authorized platform tool.
	Tool ToolInvoker
	// Emit publishes one normalized platform event.
	Emit EventSink
	// Audit records one tool call in the platform audit trail.
	Audit AdapterAuditFunc
	// Cancel reports that the adapter observed a cancellation or deadline
	// signal and stopped the agent loop. The platform remains the authority on
	// the execution terminal state.
	Cancel func(reason string)
}

// AdapterResult is the framework-neutral outcome of one adapter execution.
type AdapterResult struct {
	ExecutionID      string `json:"execution_id,omitempty"`
	Status           Status `json:"status,omitempty"`
	Output           string `json:"output,omitempty"`
	PromptTokens     int64  `json:"prompt_tokens,omitempty"`
	CompletionTokens int64  `json:"completion_tokens,omitempty"`
	TotalTokens      int64  `json:"total_tokens,omitempty"`
	ToolCallCount    int    `json:"tool_call_count,omitempty"`
	ErrorCode        string `json:"error_code,omitempty"`
	ErrorMessage     string `json:"error_message,omitempty"`
}

// EngineAdapter is the framework-neutral runtime contract. One implementation
// exists per agent framework. The platform owns everything outside this
// boundary: authorization, context resolution, tool policy, budget
// accounting, audit, event persistence and execution state.
type EngineAdapter interface {
	// ID is the stable registry key of the runtime, for example "adk" or
	// "pi". It must not contain scope, tenant or permission information.
	ID() string
	// Version is the verified runtime version that produced the recorded
	// behavior. It is stored in the execution selection snapshot.
	Version() string
	// Capabilities declares the unified capability keys this runtime can
	// honor with identical semantics.
	Capabilities() CapabilitySet
	// Execute runs one agent loop and returns the normalized result.
	Execute(ctx context.Context, request AdapterRequest, deps AdapterDeps) (AdapterResult, error)
	// Stream runs one agent loop and publishes normalized events through
	// deps.Emit while it runs.
	Stream(ctx context.Context, request AdapterRequest, deps AdapterDeps) (AdapterResult, error)
}

// AdapterRegistry holds the runtime adapters known to the process. The
// registry is code-owned and static: it is never persisted and never
// resolves scope or permission information.
type AdapterRegistry struct {
	adapters map[string]EngineAdapter
}

// NewAdapterRegistry creates an empty registry.
func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{adapters: make(map[string]EngineAdapter)}
}

// Register adds an adapter. Registering the same ID twice replaces the
// previous entry, which keeps hot reload and tests deterministic.
func (r *AdapterRegistry) Register(adapter EngineAdapter) error {
	if r == nil || adapter == nil {
		return errors.New("engine adapter is required")
	}
	id := strings.TrimSpace(adapter.ID())
	if id == "" {
		return errors.New("engine adapter id is required")
	}
	if strings.TrimSpace(adapter.Version()) == "" {
		return fmt.Errorf("engine adapter %q must declare a version", id)
	}
	if r.adapters == nil {
		r.adapters = make(map[string]EngineAdapter)
	}
	r.adapters[id] = adapter
	return nil
}

// Get returns the adapter registered under the given ID.
func (r *AdapterRegistry) Get(id string) (EngineAdapter, bool) {
	if r == nil {
		return nil, false
	}
	adapter, ok := r.adapters[strings.TrimSpace(id)]
	return adapter, ok
}

// List returns the registered adapters ordered by ID.
func (r *AdapterRegistry) List() []EngineAdapter {
	if r == nil {
		return nil
	}
	items := make([]EngineAdapter, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		items = append(items, adapter)
	}
	slices.SortFunc(items, func(left, right EngineAdapter) int {
		return strings.Compare(left.ID(), right.ID())
	})
	return items
}

// ValidateAdapterRequest performs the shared pre-flight checks for any
// adapter invocation. It keeps the platform rules identical across runtimes:
// the request must be authorized and complete, and the adapter must be able to
// honor the requested capabilities.
func ValidateAdapterRequest(request AdapterRequest, adapter EngineAdapter) error {
	if adapter == nil {
		return errors.New("engine adapter is required")
	}
	if err := request.Validate(); err != nil {
		return err
	}
	required := append([]string(nil), request.Requirements.Capabilities...)
	if request.Stream {
		required = append(required, AdapterCapabilityStreaming)
	}
	if request.Requirements.StructuredOut {
		required = append(required, AdapterCapabilityStructuredOutput)
	}
	if missing := adapter.Capabilities().Missing(required); len(missing) > 0 {
		return fmt.Errorf("engine adapter %q lacks capabilities: %s", adapter.ID(), strings.Join(missing, ", "))
	}
	return nil
}

// ContractEventCategory classifies every event type an engine execution may
// emit. The categories freeze the I008-M1 event contract:
//
//   - lifecycle: execution.started / execution.completed / execution.failed /
//     execution.cancelled, owned exclusively by the platform Runtime.
//   - planning: execution.plan.resolved, persona.resolved, context.loaded and
//     other pre-model resolution boundaries, owned by the platform.
//   - model: model.started / model.resumed, one pair per model turn.
//   - assistant: assistant.delta / assistant.progress / assistant.completed,
//     user-visible narration. Never contains hidden chain of thought.
//   - tool: tool.requested / tool.started / tool.completed / tool.failed.
//     tool.requested must precede tool.started, and exactly one of
//     tool.completed / tool.failed closes a call.
//   - maintenance: context.compacted, a context-window compaction boundary.
//     It is not a tool invocation and does not consume the tool budget.
//   - platform-extension: platform lifecycle events registered by later
//     iterations (for example I009's expert_group.resolved). Extensions are
//     registered through RegisterPlatformLifecycleEventPrefix and never change
//     the frozen categories above.
type ContractEventCategory string

const (
	ContractEventLifecycle          ContractEventCategory = "lifecycle"
	ContractEventPlanning           ContractEventCategory = "planning"
	ContractEventModel              ContractEventCategory = "model"
	ContractEventAssistant          ContractEventCategory = "assistant"
	ContractEventTool               ContractEventCategory = "tool"
	ContractEventMaintenance        ContractEventCategory = "maintenance"
	ContractEventPlatformExtension  ContractEventCategory = "platform-extension"
	ContractEventUnclassifiedEvents ContractEventCategory = "unclassified"
)

// contractEventCategories is the frozen event catalog of the I008-M1 baseline.
// Adding an entry here requires a contract review: runtimes must produce the
// same catalog or map their native events onto it.
var contractEventCategories = map[string]ContractEventCategory{
	"execution.started":       ContractEventLifecycle,
	"execution.completed":     ContractEventLifecycle,
	"execution.failed":        ContractEventLifecycle,
	"execution.cancelled":     ContractEventLifecycle,
	"execution.plan.resolved": ContractEventPlanning,
	"persona.resolved":        ContractEventPlanning,
	"context.loaded":          ContractEventPlanning,
	"model.started":           ContractEventModel,
	"model.resumed":           ContractEventModel,
	"assistant.delta":         ContractEventAssistant,
	"assistant.progress":      ContractEventAssistant,
	"assistant.completed":     ContractEventAssistant,
	"tool.requested":          ContractEventTool,
	"tool.started":            ContractEventTool,
	"tool.completed":          ContractEventTool,
	"tool.failed":             ContractEventTool,
	EventContextCompacted:     ContractEventMaintenance,
}

var platformLifecycleEventPrefixes = []string{"expert_group."}

// RegisterPlatformLifecycleEventPrefix lets a later iteration register a new
// platform lifecycle event family without touching the frozen baseline
// catalog. Registered prefixes classify as platform-extension; they are
// emitted by the platform, never by a runtime adapter, and do not alter any
// baseline assertion (joint review ruling 2 for I008/I009).
func RegisterPlatformLifecycleEventPrefix(prefix string) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return
	}
	if slices.Contains(platformLifecycleEventPrefixes, prefix) {
		return
	}
	platformLifecycleEventPrefixes = append(platformLifecycleEventPrefixes, prefix)
}

// ContractEventCategoryOf classifies an event type. Unclassified types are a
// contract violation: a runtime emitted an event the platform does not know.
func ContractEventCategoryOf(eventType string) ContractEventCategory {
	if category, ok := contractEventCategories[eventType]; ok {
		return category
	}
	for _, prefix := range platformLifecycleEventPrefixes {
		if strings.HasPrefix(eventType, prefix) {
			return ContractEventPlatformExtension
		}
	}
	return ContractEventUnclassifiedEvents
}
