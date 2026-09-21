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
	token string // a Strapi API token allowed to create roast entries, nothing more
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
