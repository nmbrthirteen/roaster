package roast

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Remote runs the audit on a server that holds the credentials, and relays the
// updates. A kiosk device never sees a model key: it carries only a terminal
// token, which does one thing, is rate limited, and can be revoked.
type Remote struct {
	URL    string
	Token  string
	Client *http.Client
}

func (r Remote) Roast(ctx context.Context, req Request, emit func(Update)) (Roast, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return Roast{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.URL, bytes.NewReader(body))
	if err != nil {
		return Roast{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	// The stand sits idle between visitors, and the far end drops the pooled
	// connection while it waits. The key marks the POST as safe to replay, so
	// the transport retries a dead reused connection instead of failing on EOF.
	httpReq.Header.Set("Idempotency-Key", rand.Text())
	if r.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.Token)
	}

	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	res, err := client.Do(httpReq)
	if err != nil {
		return Roast{}, fmt.Errorf("could not reach the roast service: %w", err)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return Roast{}, fmt.Errorf("this terminal's token was rejected; it may have been revoked")
	case http.StatusTooManyRequests:
		return Roast{}, fmt.Errorf("this stand is going faster than the roast service allows; try again in a few seconds")
	case http.StatusServiceUnavailable:
		return Roast{}, fmt.Errorf("the roast service is busy; try again in a moment")
	default:
		return Roast{}, fmt.Errorf("roast service returned %s", res.Status)
	}

	var final Roast
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimPrefix(scanner.Text(), "data: ")
		if line == "" || line == scanner.Text() {
			continue
		}
		var u Update
		if err := json.Unmarshal([]byte(line), &u); err != nil {
			continue
		}
		if u.Phase == PhaseError {
			return Roast{}, fmt.Errorf("%s", u.Error)
		}
		if u.Phase == PhaseDone && u.Roast != nil {
			final = *u.Roast
		}
		emit(u)
	}
	if err := scanner.Err(); err != nil {
		return Roast{}, err
	}
	if final.Code == "" {
		return Roast{}, fmt.Errorf("the roast service closed without finishing")
	}
	return final, nil
}

// Printed tells the service this roast's receipt came out, beside the roast
// route, so the share page's record says it was printed.
func (r Remote) Printed(ctx context.Context, code string) error {
	u, err := url.Parse(r.URL)
	if err != nil {
		return err
	}
	u.Path, u.RawQuery = path.Join(path.Dir(u.Path), "printed"), ""

	body, _ := json.Marshal(map[string]string{"code": code})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.Token)

	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("roast service returned %s", res.Status)
	}
	return nil
}
