package engine

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

const emptyModelResponseRetries = 1
const modelRetryDelay = 2 * time.Second

// ModelRetry describes one recoverable provider request failure. Attempts are
// intentionally unbounded; the execution context is the cancellation boundary.
type ModelRetry struct {
	Attempt int
	Err     error
	Delay   time.Duration
	Waiting bool
}

// retryingModel hides provider keep-alive/role/usage chunks from ADK and
// retries provider request failures until the execution is cancelled. It holds
// a streamed attempt until it completes, so a failed partial answer is never
// duplicated when the same model turn is retried.
type retryingModel struct {
	delegate   model.LLM
	onRetry    func(ModelRetry) error
	retryDelay time.Duration
}

func (m *retryingModel) Name() string { return m.delegate.Name() }

func (m *retryingModel) GenerateContent(ctx context.Context, request *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		emptyRetries := 0
		for attempt := 1; ; attempt++ {
			meaningful := false
			sawFinal := false
			var partialParts []*genai.Part
			partialHasFunctionCall := false
			responses := make([]*model.LLMResponse, 0)
			var requestErr error
			for response, err := range m.delegate.GenerateContent(ctx, request, stream) {
				if err != nil {
					requestErr = err
					break
				}
				if response == nil {
					continue
				}
				if response.ErrorCode != "" || response.ErrorMessage != "" {
					requestErr = fmt.Errorf("%s: %s", response.ErrorCode, response.ErrorMessage)
					break
				}
				responses = append(responses, response)
				if modelResponseHasMeaningfulContent(response) {
					meaningful = true
					if !response.Partial {
						sawFinal = true
					}
					if response.Partial {
						appendPartialParts(response, &partialParts, &partialHasFunctionCall)
					}
				} else if response.Partial && modelResponseHasText(response) {
					appendPartialParts(response, &partialParts, &partialHasFunctionCall)
				}
			}
			if requestErr != nil {
				if ctx.Err() != nil {
					yield(nil, ctx.Err())
					return
				}
				if retryErr := m.retry(ctx, attempt, requestErr); retryErr != nil {
					yield(nil, retryErr)
					return
				}
				continue
			}
			if stream && meaningful && !sawFinal && len(partialParts) > 0 && !partialHasFunctionCall {
				responses = append(responses, &model.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: partialParts}, TurnComplete: true})
			}
			if meaningful {
				if !yieldMeaningfulResponses(responses, yield) {
					return
				}
				return
			}
			if emptyRetries >= emptyModelResponseRetries || ctx.Err() != nil {
				return
			}
			emptyRetries++
			if retryErr := m.retry(ctx, attempt, errors.New("模型未返回可展示内容")); retryErr != nil {
				yield(nil, retryErr)
				return
			}
		}
	}
}

func (m *retryingModel) retry(ctx context.Context, attempt int, cause error) error {
	delay := m.retryDelay
	if delay <= 0 {
		delay = modelRetryDelay
	}
	if m.onRetry != nil {
		if err := m.onRetry(ModelRetry{Attempt: attempt, Err: cause, Delay: delay, Waiting: true}); err != nil {
			return err
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	if m.onRetry != nil {
		if err := m.onRetry(ModelRetry{Attempt: attempt, Err: cause, Delay: delay}); err != nil {
			return err
		}
	}
	return nil
}

func yieldMeaningfulResponses(responses []*model.LLMResponse, yield func(*model.LLMResponse, error) bool) bool {
	meaningful := false
	pendingWhitespace := make([]*model.LLMResponse, 0)
	for _, response := range responses {
		if response == nil {
			continue
		}
		if modelResponseHasMeaningfulContent(response) {
			if response.Partial {
				for _, pending := range pendingWhitespace {
					if !yield(pending, nil) {
						return false
					}
				}
				pendingWhitespace = nil
			}
			meaningful = true
			if !yield(response, nil) {
				return false
			}
			continue
		}
		if response.Partial && modelResponseHasText(response) {
			if meaningful {
				if !yield(response, nil) {
					return false
				}
			} else {
				pendingWhitespace = append(pendingWhitespace, response)
			}
		}
	}
	return true
}

func appendPartialParts(response *model.LLMResponse, parts *[]*genai.Part, hasFunctionCall *bool) {
	if response == nil || response.Content == nil {
		return
	}
	for _, part := range response.Content.Parts {
		if part == nil {
			continue
		}
		*hasFunctionCall = *hasFunctionCall || part.FunctionCall != nil
		if part.Text == "" || part.Thought {
			continue
		}
		copy := *part
		*parts = append(*parts, &copy)
	}
}

func modelResponseHasMeaningfulContent(response *model.LLMResponse) bool {
	if response == nil || response.Content == nil {
		return false
	}
	for _, part := range response.Content.Parts {
		if part == nil {
			continue
		}
		if (!part.Thought && strings.TrimSpace(part.Text) != "") ||
			part.FunctionCall != nil ||
			part.FunctionResponse != nil ||
			part.CodeExecutionResult != nil ||
			part.ExecutableCode != nil ||
			part.FileData != nil ||
			part.InlineData != nil ||
			part.ToolCall != nil ||
			part.ToolResponse != nil ||
			part.AudioTranscription != nil {
			return true
		}
	}
	return false
}

func modelResponseHasText(response *model.LLMResponse) bool {
	if response == nil || response.Content == nil {
		return false
	}
	for _, part := range response.Content.Parts {
		if part != nil && !part.Thought && part.Text != "" {
			return true
		}
	}
	return false
}

var _ model.LLM = (*retryingModel)(nil)
