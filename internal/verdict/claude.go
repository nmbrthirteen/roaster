package verdict

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Writer turns a brief into the written page. What comes back is raw: Merge
// decides which of its lines print. The interface is here so everything
// around it can be tested without a network.
type Writer interface {
	Write(ctx context.Context, b Brief) (Page, error)
}

var (
	// ErrDeclined is the model choosing not to write this one. Nothing is
	// retried against another model: a refusal on a roast is a signal to be
	// tamer, and Fallback is tamer.
	ErrDeclined = errors.New("the model declined to write this roast")

	// ErrUnusable is a reply that came back and cannot be read.
	ErrUnusable = errors.New("the model's reply could not be read")
)

// ClaudeModel is Claude's default. A roast lives or dies on being specific and
// on timing, which is where the most capable model earns its place; low effort
// keeps the thinking short, because a queue is waiting.
const ClaudeModel = "claude-opus-5"

type Claude struct {
	client anthropic.Client
	model  string
}

// NewClaude reads its key from ANTHROPIC_API_KEY, as the SDK does, and nowhere
// else. One retry covers a dropped connection; more than that and the visitor
// is better served by Fallback than by waiting.
func NewClaude(opts ...option.RequestOption) Claude {
	opts = append([]option.RequestOption{option.WithMaxRetries(1)}, opts...)
	return Claude{client: anthropic.NewClient(opts...), model: ClaudeModel}
}

func (c Claude) Write(ctx context.Context, b Brief) (Page, error) {
	started := time.Now()

	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 8192,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortLow,
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(b.Render() + ask)),
		},
	})
	if err != nil {
		return Page{}, err
	}

	slog.Info("verdict",
		"model", resp.Model,
		"stop", resp.StopReason,
		"input_tokens", resp.Usage.InputTokens,
		"output_tokens", resp.Usage.OutputTokens,
		"ms", time.Since(started).Milliseconds(),
	)

	if resp.StopReason == anthropic.StopReasonRefusal {
		return Page{}, ErrDeclined
	}

	var text strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	return parse(text.String())
}
