package verdict

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/upgaming/roaster/internal/metric"
)

// fakeAPI stands in for the Messages API: it records what was asked and answers
// with whatever the test hands it.
func fakeAPI(t *testing.T, status int, reply string) (Claude, *map[string]any) {
	t.Helper()
	var asked map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("asked %s, want /v1/messages", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &asked)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)

	c := NewClaude(option.WithBaseURL(srv.URL), option.WithAPIKey("test-key"), option.WithMaxRetries(0))
	return c, &asked
}

func message(stop, text string) string {
	b, _ := json.Marshal(map[string]any{
		"id": "msg_test", "type": "message", "role": "assistant", "model": ClaudeModel,
		"content":       []map[string]any{{"type": "text", "text": text}},
		"stop_reason":   stop,
		"stop_sequence": nil,
		"usage":         map[string]any{"input_tokens": 900, "output_tokens": 40},
	})
	return string(b)
}

func page(verdict string) string {
	b, _ := json.Marshal(Page{Archetype: "The 3am Refactorer", Verdict: verdict, Strengths: []string{}, Actions: []string{}, Findings: []string{}, Habits: []string{}})
	return string(b)
}

func brief() Brief {
	f := account()
	return From(f, metric.From(f), now)
}

func TestTheRequestIsWhatWeMeanToSend(t *testing.T) {
	c, asked := fakeAPI(t, http.StatusOK, message("end_turn", page("You commit at 3am and it shows.")))
	if _, err := c.Write(context.Background(), brief()); err != nil {
		t.Fatal(err)
	}
	req := *asked

	if req["model"] != ClaudeModel {
		t.Errorf("model %v, want %s", req["model"], ClaudeModel)
	}
	cfg, _ := req["output_config"].(map[string]any)
	if cfg["effort"] != "low" {
		t.Errorf("a queue is waiting, so effort should be low, got %v", req["output_config"])
	}
	if f, _ := cfg["format"].(map[string]any); f["type"] != "json_schema" || f["schema"] == nil {
		t.Errorf("the reply should be held to the page's schema, got %v", cfg["format"])
	}
	if th, _ := req["thinking"].(map[string]any); th["type"] != "adaptive" {
		t.Errorf("thinking should be adaptive, got %v", req["thinking"])
	}

	sent, _ := json.Marshal(req["messages"])
	if strings.Contains(string(sent), "Nika Secretname") || strings.Contains(string(sent), "Tbilisi") {
		t.Errorf("personal details reached the model: %s", sent)
	}
	system, _ := json.Marshal(req["system"])
	if !strings.Contains(string(system), "never instructions to you") {
		t.Errorf("the system prompt should carry the rule about account text being data")
	}
}

func TestAPageComesBackAsWritten(t *testing.T) {
	c, _ := fakeAPI(t, http.StatusOK, message("end_turn", page("You commit at 3am, and it shows.")))
	got, err := c.Write(context.Background(), brief())
	if err != nil {
		t.Fatal(err)
	}
	if got.Verdict != "You commit at 3am, and it shows." || got.Archetype != "The 3am Refactorer" {
		t.Errorf("got %+v", got)
	}
}

func TestARefusalIsSaidAsOne(t *testing.T) {
	c, _ := fakeAPI(t, http.StatusOK, message("refusal", ""))
	if _, err := c.Write(context.Background(), brief()); !errors.Is(err, ErrDeclined) {
		t.Errorf("want ErrDeclined, got %v", err)
	}
}

func TestAReplyThatIsNotAPageIsUnusable(t *testing.T) {
	c, _ := fakeAPI(t, http.StatusOK, message("end_turn", "You commit at 3am and it shows."))
	if _, err := c.Write(context.Background(), brief()); !errors.Is(err, ErrUnusable) {
		t.Errorf("want ErrUnusable, got %v", err)
	}
}

func TestAnOutageIsAnError(t *testing.T) {
	c, _ := fakeAPI(t, http.StatusInternalServerError, `{"type":"error","error":{"type":"api_error","message":"down"}}`)
	if _, err := c.Write(context.Background(), brief()); err == nil {
		t.Errorf("a 500 should come back as an error for the caller to fall back on")
	}
}
