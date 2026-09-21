package verdict

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	openAIEndpoint = "https://api.openai.com/v1/responses"

	// OpenAIModel is OpenAI's default: the smallest of the family. The verdict
	// is one short line from a small brief, so the cost per roast is a
	// fraction of a cent. gpt-5.6-terra or gpt-5.6-sol go in OPENAI_MODEL if
	// the jokes need more.
	OpenAIModel = "gpt-5.6-luna"
)

// OpenAI writes the verdict through the Responses API. It is plain HTTP rather
// than an SDK: the whole exchange is one POST, and the GitHub client in this
// repository already works the same way.
type OpenAI struct {
	Key   string       // from OPENAI_API_KEY; never logged
	Model string       // empty is OpenAIModel
	URL   string       // empty is OpenAI; tests set it
	HTTP  *http.Client // nil is one with no timeout of its own; the caller's context bounds it
}

type openAIRequest struct {
	Model        string `json:"model"`
	Instructions string `json:"instructions"`
	Input        string `json:"input"`
	MaxOutput    int    `json:"max_output_tokens"`
	Reasoning    struct {
		Effort string `json:"effort"`
	} `json:"reasoning"`
	Text struct {
		Format openAIFormat `json:"format"`
	} `json:"text"`

	// Store is always false. What a visitor's account says is not kept on
	// OpenAI's side past the reply.
	Store bool `json:"store"`
}

type openAIFormat struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Schema map[string]any `json:"schema"`
	Strict bool           `json:"strict"`
}

type openAIResponse struct {
	Model      string `json:"model"`
	Status     string `json:"status"`
	Incomplete *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Refusal string `json:"refusal"`
		} `json:"content"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type openAIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (o OpenAI) Write(ctx context.Context, b Brief) (Page, error) {
	if o.Key == "" {
		return Page{}, errors.New("no OpenAI key")
	}
	model := o.Model
	if model == "" {
		model = OpenAIModel
	}

	req := openAIRequest{
		Model:        model,
		Instructions: system,
		Input:        b.Render() + ask,
		MaxOutput:    8192,
	}
	req.Reasoning.Effort = "low"
	req.Text.Format = openAIFormat{Type: "json_schema", Name: "roast", Schema: schema, Strict: true}
	body, err := json.Marshal(req)
	if err != nil {
		return Page{}, err
	}

	started := time.Now()
	resp, err := o.post(ctx, body)
	if err != nil {
		return Page{}, err
	}

	slog.Info("verdict",
		"provider", "openai",
		"model", resp.Model,
		"status", resp.Status,
		"input_tokens", resp.Usage.InputTokens,
		"output_tokens", resp.Usage.OutputTokens,
		"ms", time.Since(started).Milliseconds(),
	)

	// A refusal can arrive as its own content item, or as a reply cut short
	// by the content filter. Either way the answer is the tamer line, and
	// no reason string is relied on beyond the word that names the filter.
	var text strings.Builder
	for _, item := range resp.Output {
		if item.Type != "message" {
			continue
		}
		for _, c := range item.Content {
			switch c.Type {
			case "refusal":
				return Page{}, ErrDeclined
			case "output_text":
				text.WriteString(c.Text)
			}
		}
	}
	if resp.Incomplete != nil && strings.Contains(resp.Incomplete.Reason, "content") {
		return Page{}, ErrDeclined
	}
	return parse(text.String())
}

// post sends the request, trying once more on a dropped connection, a rate
// limit or a server error. More than that and the visitor is better served by
// the numbers than by waiting.
func (o OpenAI) post(ctx context.Context, body []byte) (openAIResponse, error) {
	url := o.URL
	if url == "" {
		url = openAIEndpoint
	}
	client := o.HTTP
	if client == nil {
		client = &http.Client{}
	}

	var last error
	for attempt := range 2 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return openAIResponse{}, ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return openAIResponse{}, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+o.Key)

		res, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return openAIResponse{}, ctx.Err()
			}
			last = fmt.Errorf("could not reach OpenAI: %w", err)
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if err != nil {
			last = err
			continue
		}

		if res.StatusCode == http.StatusOK {
			var out openAIResponse
			if err := json.Unmarshal(raw, &out); err != nil {
				return openAIResponse{}, fmt.Errorf("OpenAI sent something unreadable: %w", err)
			}
			return out, nil
		}

		// The error body says what went wrong. The key is never in it, and it
		// never goes anywhere but the log.
		var e openAIError
		json.Unmarshal(raw, &e)
		last = fmt.Errorf("OpenAI returned %s: %s %s", res.Status, e.Error.Type, e.Error.Message)

		if res.StatusCode != http.StatusTooManyRequests && res.StatusCode < 500 {
			return openAIResponse{}, last
		}
	}
	return openAIResponse{}, last
}
