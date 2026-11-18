package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tekurischema "github.com/santhosh-tekuri/jsonschema/v6"
)

type executionState struct {
	budget           *executionBudget
	mu               sync.RWMutex
	observations     map[string]any
	observationBytes int
	observationIndex map[string]compactedObservation
	compactedIDs     map[string]struct{}
	compacted        []compactedObservation
	confirmedFacts   []string
	hypotheses       []string
	eliminatedCauses []string
	openQuestions    []string
	nextActions      []string
}

// compactedObservation is a bounded, evidence-backed entry retained after an
// older tool-call turn leaves the provider context window. It intentionally
// contains no model-authored conclusion: exact output remains available only
// through read_observation during this execution.
type compactedObservation struct {
	ObservationID string `json:"observation_id"`
	Tool          string `json:"tool"`
	Outcome       string `json:"outcome"`
	Summary       string `json:"summary"`
}

func newExecutionState() *executionState {
	return &executionState{
		observations:     make(map[string]any),
		observationIndex: make(map[string]compactedObservation),
		compactedIDs:     make(map[string]struct{}),
	}
}

func (s *executionState) save(value any) (string, bool) {
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) > maxObservationStoreBytes {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.observationBytes+len(encoded) > maxObservationStoreBytes {
		return "", false
	}
	if s.budget != nil {
		for {
			used := s.budget.observationBytes.Load()
			if used+int64(len(encoded)) > maxObservationStoreBytes {
				return "", false
			}
			if s.budget.observationBytes.CompareAndSwap(used, used+int64(len(encoded))) {
				break
			}
		}
	}
	id := fmt.Sprintf("obs-%d", time.Now().UnixNano())
	s.observations[id] = value
	s.observationBytes += len(encoded)
	return id, true
}

func (s *executionState) read(id string, offset, limit int) (any, bool) {
	s.mu.RLock()
	value, ok := s.observations[id]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	encoded, _ := json.Marshal(value)
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 8000
	}
	if offset > len(encoded) {
		offset = len(encoded)
	}
	end := offset + limit
	if end > len(encoded) {
		end = len(encoded)
	}
	return map[string]any{"observation_id": id, "offset": offset, "next_offset": end, "done": end >= len(encoded), "content": string(encoded[offset:end])}, true
}

func (s *executionState) observe(toolName, observationID string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	text := compactObservationText(value)
	if text == "" {
		return
	}
	outcome := "success"
	if strings.Contains(strings.ToLower(text), "error") || strings.Contains(strings.ToLower(text), "failed") {
		outcome = "error"
		s.openQuestions = appendBounded(s.openQuestions, toolName+": 返回了错误，需要重新评估")
		s.eliminatedCauses = appendBounded(s.eliminatedCauses, toolName+": 当前调用路径不可用")
	} else {
		s.confirmedFacts = appendBounded(s.confirmedFacts, toolName+": 已获得环境观察")
	}
	if observationID != "" {
		s.observationIndex[observationID] = compactedObservation{
			ObservationID: observationID,
			Tool:          toolName,
			Outcome:       outcome,
			Summary:       text,
		}
	}
}

func (s *executionState) markCompacted(observationIDs []string) []compactedObservation {
	if len(observationIDs) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	added := make([]compactedObservation, 0, len(observationIDs))
	for _, observationID := range observationIDs {
		if _, alreadyCompacted := s.compactedIDs[observationID]; alreadyCompacted {
			continue
		}
		item, known := s.observationIndex[observationID]
		if !known {
			continue
		}
		s.compactedIDs[observationID] = struct{}{}
		s.compacted = appendBoundedCompacted(s.compacted, item)
		added = append(added, item)
	}
	return added
}

func (s *executionState) prompt() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	encode := func(items []string) string { b, _ := json.Marshal(items); return string(b) }
	compacted, _ := json.Marshal(s.compacted)
	return "Execution state (machine-maintained, do not expose as chain of thought): confirmed_facts=" + encode(s.confirmedFacts) + ", hypotheses=" + encode(s.hypotheses) + ", eliminated_causes=" + encode(s.eliminatedCauses) + ", open_questions=" + encode(s.openQuestions) + ", next_actions=" + encode(s.nextActions) + ", compacted_observations=" + string(compacted) + ". Compacted observations are evidence references, not conclusions; use read_observation(observation_id, offset, limit) when exact details are required."
}

func appendBounded(items []string, value string) []string {
	if len(items) > 12 {
		items = items[len(items)-12:]
	}
	return append(items, value)
}

func appendBoundedCompacted(items []compactedObservation, value compactedObservation) []compactedObservation {
	const maxCompactedObservations = 16
	if len(items) >= maxCompactedObservations {
		items = items[len(items)-maxCompactedObservations+1:]
	}
	return append(items, value)
}

func compactObservationText(value any) string {
	summary := summarizeObservation(value)
	encoded, err := json.Marshal(summary)
	if err != nil {
		return "工具返回了无法展示的结果"
	}
	return safeProgressText(string(encoded), 480)
}

// AgentRunner is the Skill-independent Agent execution runtime. It owns the
// model call, ADK reasoning loop, generic context tools, budgets and output
// contract validation. Skill is represented only by optional fields on
// Request (instruction, schemas and tool declarations are supplied by the
// Skill adapter when present).
type AgentRunner struct {
	BuildModel ModelBuilder
	AppName    string
}

func NewAgentRunner(buildModel ModelBuilder) *AgentRunner {
	return &AgentRunner{BuildModel: buildModel, AppName: "opsseek-engine"}
}

func (r *AgentRunner) Run(ctx context.Context, request Request) (Result, error) {
	return r.execute(ctx, request, nil)
}

func (r *AgentRunner) RunStream(ctx context.Context, request Request, sink EventSink) (Result, error) {
	// The streaming entry point is an explicit contract; callers should not
	// have to duplicate the flag in Request to receive provider deltas.
	request.Stream = true
	return r.execute(ctx, request, sink)
}

// emitModelResumed marks the barrier between a completed tool batch and the
// next model turn. It intentionally emits one event for the whole batch: ADK
// executes same-turn independent calls concurrently, and the model should
// reason over all of their observations together. A single-call payload keeps
// the historical fields for clients that only render one tool; multi-call
// payloads additionally expose the ordered observation list and parallel flag.
func emitModelResumed(sink EventSink, executionID string, iteration int, observations []map[string]any) {
	if sink == nil || len(observations) == 0 {
		return
	}
	anyError := false
	allError := true
	for _, observation := range observations {
		if strings.EqualFold(fmt.Sprint(observation["outcome"]), "error") {
			anyError = true
		} else {
			allError = false
		}
	}
	outcome := "success"
	if allError {
		outcome = "error"
	} else if anyError {
		outcome = "partial"
	}
	payload := map[string]any{
		"iteration":    iteration,
		"tool_count":   len(observations),
		"parallel":     len(observations) > 1,
		"observations": observations,
		"outcome":      outcome,
	}
	if len(observations) == 1 {
		for _, key := range []string{"tool", "model_tool", "resource_id", "call_id", "observation"} {
			if value, ok := observations[0][key]; ok {
				payload[key] = value
			}
		}
	} else {
		payload["observation"] = map[string]any{"tools": observations, "count": len(observations)}
	}
	_ = sink(Event{ExecutionID: executionID, Type: "model.resumed", Status: StatusRunning, Payload: payload})
}

func agentToolAllowed(profile *Persona, declared []string, restricted bool, name string) bool {
	if len(declared) > 0 {
		allowed := false
		for _, item := range declared {
			if strings.TrimSpace(item) == name {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	} else if restricted {
		return false
	}
	if profile == nil || len(profile.AllowedTools) == 0 {
		return true
	}
	for _, allowed := range profile.AllowedTools {
		if strings.TrimSpace(allowed) == name {
			return true
		}
	}
	return false
}

func resolvedToolBinding(request Request, name string) toolBinding {
	for _, binding := range buildToolBindings(resolvedToolDefinitions(request)) {
		if binding.modelName == name {
			return binding
		}
	}
	return toolBinding{}
}

func resolvedToolDefinitions(request Request) []ToolDefinition {
	if request.ResolvedContext == nil {
		return nil
	}
	return request.ResolvedContext.Tools
}

func executionPrompt(request Request) (string, error) {
	payload := make(map[string]any, len(request.Input)+4)
	for key, value := range request.Input {
		if isModelInternalIDField(key) {
			continue
		}
		payload[key] = StripModelInternalIDs(value)
	}
	if request.Task != "" {
		payload["task"] = request.Task
	}
	if len(request.Messages) > 0 {
		payload["conversation"] = request.Messages
	}
	// Context providers may collect a small, read-only snapshot while resolving
	// the selected resources (for example a PostgreSQL health check or MCP
	// server metadata). Make that environment feedback available on the first
	// model turn as well as subsequent tool observations. It is normalized,
	// redacted and bounded by summarizeObservation because connector/MCP data
	// is untrusted and may contain credentials or unexpectedly large payloads.
	if request.ResolvedContext != nil && len(request.ResolvedContext.Facts) > 0 {
		payload["environment_facts"] = StripModelInternalIDs(summarizeObservation(request.ResolvedContext.Facts))
	}
	if request.ResolvedContext != nil && len(request.ResolvedContext.Resources) > 0 {
		selectedResources := make([]map[string]string, 0, len(request.ResolvedContext.Resources))
		for _, resource := range request.ResolvedContext.Resources {
			selected := map[string]string{}
			if name := strings.TrimSpace(resource.Name); name != "" {
				selected["name"] = name
			}
			if kind := strings.TrimSpace(resource.Kind); kind != "" {
				selected["kind"] = kind
			}
			if subtype := strings.TrimSpace(resource.Subtype); subtype != "" {
				selected["subtype"] = subtype
			}
			if len(selected) > 0 {
				selectedResources = append(selectedResources, selected)
			}
		}
		if len(selectedResources) > 0 {
			payload["selected_resources"] = selectedResources
		}
	}
	if len(payload) == 0 {
		return "请完成当前任务。", nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("Engine input must be JSON serializable: %w", err)
	}
	return string(encoded), nil
}

func failedResult(executionID string, calls int64, ctx context.Context, err error) (Result, error) {
	code := "agent"
	status := StatusFailed
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		code, status = "timeout", StatusCancelled
	} else if errors.Is(ctx.Err(), context.Canceled) {
		code, status = "cancelled", StatusCancelled
	} else if errors.Is(err, context.DeadlineExceeded) {
		code = "provider_timeout"
	} else if errors.Is(err, context.Canceled) {
		code = "provider_interrupted"
	}
	return Result{ExecutionID: executionID, Status: status, ErrorCode: code, ErrorMessage: publicError(err), ToolCallCount: int(calls)}, err
}

// toolDecisionProgressText keeps tool-call progress useful without exposing
// function names or model-private reasoning when the model emits no narration.
func toolDecisionProgressText(names []string) string {
	labels := map[string]string{
		"repository_tree":     "读取代码仓库目录结构",
		"repository_file":     "读取代码文件内容",
		"repository_search":   "检索代码仓库内容",
		"repository_branches": "读取代码分支和提交信息",
		"repository_status":   "检查代码仓库状态",
		"repository_metadata": "读取代码仓库元数据",
	}
	actions := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.SplitN(strings.ToLower(strings.TrimSpace(name)), "__resource_", 2)[0]
		if label := labels[name]; label != "" && !slices.Contains(actions, label) {
			actions = append(actions, label)
		}
	}
	switch len(actions) {
	case 1:
		return "正在" + actions[0] + "。"
	case 2, 3:
		return "正在并行" + strings.Join(actions, "、") + "。"
	default:
		return "正在调用已授权资源收集当前任务所需的信息。"
	}
}

func missingCapabilities(actual, required []string) []string {
	missing := make([]string, 0)
	for _, value := range required {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains(actual, value) || slices.Contains(missing, value) {
			continue
		}
		missing = append(missing, value)
	}
	return missing
}

func validateJSON(schemaRaw json.RawMessage, value []byte) error {
	var schemaValue any
	if err := json.Unmarshal(schemaRaw, &schemaValue); err != nil {
		return err
	}
	var data any
	if err := json.Unmarshal(value, &data); err != nil {
		return errors.New("value must be valid JSON")
	}
	compiler := tekurischema.NewCompiler()
	if err := compiler.AddResource("memory://engine-schema.json", schemaValue); err != nil {
		return err
	}
	schema, err := compiler.Compile("memory://engine-schema.json")
	if err != nil {
		return err
	}
	return schema.Validate(data)
}

func publicError(err error) string {
	if err == nil {
		return ""
	}
	message := scrubVisibleText(strings.TrimSpace(err.Error()))
	return truncateUTF8(message, 1000)
}

// SafeErrorMessage returns the bounded, credential-redacted form suitable for
// API responses and user-visible execution events. Internal callers should
// still retain the original error for classification and logging.
func SafeErrorMessage(err error) string {
	return publicError(err)
}

func classifyToolResponseError(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "not found"):
		return "tool_not_found"
	case strings.Contains(lower, "denied"), strings.Contains(lower, "forbidden"):
		return "tool_denied"
	case strings.Contains(lower, "invalid"), strings.Contains(lower, "argument"), strings.Contains(lower, "required"), strings.Contains(lower, "validating"):
		return "tool_validation"
	default:
		return "tool_execution"
	}
}

// summarizeObservation creates safe, bounded environment feedback for the UI
// and the next model turn. Tool output is untrusted and may contain secrets or
// very large logs, so redact recursively and cap the serialized representation.
func summarizeObservation(value any) any {
	// Normalize through JSON so typed maps/slices returned by an MCP adapter
	// receive the same recursive redaction as map[string]any values.
	normalized := value
	if encoded, err := json.Marshal(value); err == nil {
		var generic any
		if json.Unmarshal(encoded, &generic) == nil {
			normalized = generic
		}
	}
	redacted := scrubObservationText(redactValue(normalized))
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return "工具返回了无法展示的结果"
	}
	if len(encoded) <= maxObservationBytes {
		return redacted
	}
	if object, ok := redacted.(map[string]any); ok {
		// Preserve diagnostic signal before verbose payloads such as logs or
		// query rows. The omitted fields remain available through observation
		// paging when the model needs exact details.
		priority := make(map[string]any)
		for key, item := range object {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "error") || strings.Contains(lower, "status") || strings.Contains(lower, "state") || strings.Contains(lower, "count") || strings.Contains(lower, "total") || strings.Contains(lower, "message") || strings.Contains(lower, "duration") {
				priority[key] = item
			}
		}
		if len(priority) > 0 {
			encodedPriority, _ := json.Marshal(priority)
			if len(encodedPriority) < maxObservationBytes {
				priority["details_truncated"] = true
				return priority
			}
		}
	}
	return map[string]any{"summary": truncateBytesUTF8(string(encoded), maxObservationBytes), "truncated": true}
}

// Tool observations are bounded independently from the provider output budget.
// The larger bound allows docker logs and similar diagnostics to retain a
// useful 200-line window while still preventing an unbounded tool response from
// consuming the model context. Exact values remain available through the
// read_observation paging tool.
const maxObservationBytes = 256 << 10
const maxObservationTextRunes = 64000
const maxObservationStoreBytes = 32 << 20

var sensitiveProgressPattern = regexp.MustCompile(`(?i)(\b(?:bearer|token|password|secret|credential|api[\s_-]*key|authorization|private[\s_-]*key|access[\s_-]*key|client[\s_-]*secret)\b\s*(?:[:=]\s*))(?:bearer\s+)?[^\s,;]+`)
var sensitiveJSONPattern = regexp.MustCompile(`(?i)(["']?(?:bearer|token|password|secret|credential|api[\s_-]*key|authorization|private[\s_-]*key|access[\s_-]*key|client[\s_-]*secret)["']?\s*:\s*)("(?:\\.|[^"\\])*"|[^,\s}\]]+)`)

func safeProgressText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = sensitiveProgressPattern.ReplaceAllString(value, `$1[REDACTED]`)
	return truncateUTF8(value, maxRunes)
}

func safeStreamingDelta(value string, maxRunes int) string {
	// Whitespace is meaningful in Markdown (for example the space after `##`
	// and the blank line separating a heading from its paragraph). Only an
	// actually empty provider chunk may be discarded.
	if value == "" {
		return ""
	}
	value = sensitiveProgressPattern.ReplaceAllString(value, `$1[REDACTED]`)
	if maxRunes <= 0 {
		return value
	}
	return truncateUTF8(value, maxRunes)
}

func scrubVisibleText(value string) string {
	value = sensitiveProgressPattern.ReplaceAllString(value, `$1[REDACTED]`)
	value = sensitiveJSONPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := sensitiveJSONPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return `[REDACTED]`
		}
		replacement := `[REDACTED]`
		if strings.HasPrefix(strings.TrimSpace(parts[2]), `"`) {
			replacement = `"[REDACTED]"`
		}
		return parts[1] + replacement
	})
	return value
}

func truncateUTF8(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}

func truncateBytesUTF8(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len([]byte(value)) <= maxBytes {
		return value
	}
	ellipsis := []byte("…")
	if maxBytes < len(ellipsis) {
		bytes := []byte(value)
		limit := maxBytes
		for limit > 0 && !utf8.Valid(bytes[:limit]) {
			limit--
		}
		return string(bytes[:limit])
	}
	limit := maxBytes - len(ellipsis)
	bytes := []byte(value)
	for limit > 0 && !utf8.Valid(bytes[:limit]) {
		limit--
	}
	return string(bytes[:limit]) + string(ellipsis)
}

func scrubObservationText(value any) any {
	switch item := value.(type) {
	case string:
		return safeProgressText(item, maxObservationTextRunes)
	case map[string]any:
		result := make(map[string]any, len(item))
		for key, nested := range item {
			result[key] = scrubObservationText(nested)
		}
		return result
	case []any:
		result := make([]any, len(item))
		for index, nested := range item {
			result[index] = scrubObservationText(nested)
		}
		return result
	default:
		return value
	}
}
