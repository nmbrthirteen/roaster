package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/upgaming/roaster/internal/roast"
)

// archive saves every finished roast to Strapi, which serves the page the
// receipt's QR code points at. It never holds the stand up: the entry is sent
// after the visitor has their roast, and a slow or failing Strapi costs the
// share page, not the receipt.
type archive struct {
	next  roast.Provider
	url   string // Strapi's base URL
	token string // a Strapi API token allowed to create roast entries and mark them printed
	http  *http.Client
}

type entry struct {
	Code      string          `json:"code"`
	Handle    string          `json:"handle"`
	Pack      string          `json:"pack,omitempty"`
	Event     string          `json:"event,omitempty"`
	Terminal  string          `json:"terminal,omitempty"`
	Score     string          `json:"score"`
	ScoreTag  string          `json:"scoreTag"`
	Archetype string          `json:"archetype,omitempty"`
	Verdict   string          `json:"verdict"`
	Metrics   []roast.Metric  `json:"metrics"`
	Actions   []string        `json:"actions"`
	Strengths []string        `json:"strengths,omitempty"`
	Findings  []roast.Finding `json:"findings,omitempty"`
	Habits    []roast.Finding `json:"habits,omitempty"`
	Story     []roast.Section `json:"story,omitempty"`
	Exhibit   *roast.Item     `json:"exhibit,omitempty"`
	Heat      [][7]int        `json:"heat,omitempty"`
	Feed      []roast.Item    `json:"feed,omitempty"`
}

func (a archive) Roast(ctx context.Context, req roast.Request, emit func(roast.Update)) (roast.Roast, error) {
	var feed []roast.Item
	r, err := a.next.Roast(ctx, req, func(u roast.Update) {
		if u.Phase == roast.PhaseFeed {
			feed = u.Feed
		}
		emit(u)
	})
	if err != nil {
		return r, err
	}
	go a.save(entry{
		Code: r.Code, Handle: r.Handle, Pack: r.Pack, Event: req.Event, Terminal: req.Terminal,
		Score: r.Score, ScoreTag: r.ScoreTag, Archetype: r.Archetype, Verdict: r.Verdict, Metrics: r.Metrics,
		Actions: r.Actions, Strengths: r.Strengths, Findings: r.Findings, Habits: r.Habits, Story: r.Story,
		Exhibit: r.Exhibit, Heat: r.Heat, Feed: feed,
	})
	return r, nil
}

func (a archive) save(e entry) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, reply, err := a.post(ctx, e)
	if err == nil && status == http.StatusBadRequest && e.Archetype != "" && strings.Contains(reply, "archetype") {
		slog.Warn("archive", "code", e.Code, "err", "Strapi has no archetype field; saved without it")
		e.Archetype = ""
		status, _, err = a.post(ctx, e)
	}
	if err != nil {
		slog.Error("archive", "code", e.Code, "err", err)
		return
	}
	if status != http.StatusOK && status != http.StatusCreated {
		slog.Error("archive", "code", e.Code, "err", fmt.Sprintf("Strapi answered %d", status))
		return
	}
	slog.Info("archive", "code", e.Code)
}

func (a archive) post(ctx context.Context, e entry) (int, string, error) {
	body, err := json.Marshal(map[string]entry{"data": e})
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(a.url, "/")+"/api/roast-entries", bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.token)

	client := a.http
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer res.Body.Close()
	reply, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
	return res.StatusCode, string(reply), nil
}

// printedRetries is how long to wait before each new try at marking a receipt
// printed. The entry is saved after the roast, so a quick print can land first.
var printedRetries = []time.Duration{3 * time.Second, 10 * time.Second, 30 * time.Second}

// markPrinted tells Strapi a stand printed this roast's receipt.
func (a archive) markPrinted(code string) {
	for try := 0; ; try++ {
		status, err := a.call(http.MethodPost, "/api/roaster/entries/"+code+"/printed")
		switch {
		case err == nil && status < 300:
			slog.Info("printed", "code", code)
			return
		case err == nil && status != http.StatusNotFound:
			slog.Error("printed", "code", code, "err", fmt.Sprintf("Strapi answered %d", status))
			return
		case try == len(printedRetries):
			slog.Error("printed", "code", code, "err", fmt.Sprint("gave up: ", errOr(err, "no entry")))
			return
		}
		time.Sleep(printedRetries[try])
	}
}

func (a archive) call(method, path string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(a.url, "/")+path, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	client := a.http
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	res.Body.Close()
	return res.StatusCode, nil
}

func errOr(err error, fallback string) string {
	if err != nil {
		return err.Error()
	}
	return fallback
}
