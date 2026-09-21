package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/upgaming/roaster/internal/roast"
)

func TestEveryRoastIsSavedForItsSharePage(t *testing.T) {
	got := make(chan map[string]entry, 1)
	strapi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/roast-entries" || r.Header.Get("Authorization") != "Bearer strapi-token" {
			t.Errorf("unexpected request %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		var body map[string]entry
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		got <- body
	}))
	defer strapi.Close()

	a := archive{next: roast.Demo{}, url: strapi.URL + "/", token: "strapi-token"}
	r, err := a.Roast(context.Background(), roast.Request{Handle: "octocat", Event: "tbilisi-dev-days", Terminal: "007"}, func(roast.Update) {})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case body := <-got:
		e := body["data"]
		if e.Code != r.Code || e.Handle != "octocat" || e.Event != "tbilisi-dev-days" || e.Terminal != "007" {
			t.Errorf("the entry should carry the roast and where it was made, got %+v", e)
		}
		if len(e.Feed) == 0 || e.Verdict == "" || len(e.Metrics) == 0 {
			t.Errorf("the share page needs the feed, verdict and gauges, got %+v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the roast was never sent to Strapi")
	}
}

func TestAFailingStrapiNeverCostsTheReceipt(t *testing.T) {
	strapi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer strapi.Close()

	a := archive{next: roast.Demo{}, url: strapi.URL, token: "t"}
	if _, err := a.Roast(context.Background(), roast.Request{Handle: "octocat"}, func(roast.Update) {}); err != nil {
		t.Errorf("the visitor should still get their roast, got %v", err)
	}
}

func TestAnOlderStrapiStillGetsTheRoast(t *testing.T) {
	var sent []entry
	strapi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]entry
		json.NewDecoder(r.Body).Decode(&body)
		sent = append(sent, body["data"])
		if body["data"].Archetype != "" {
			http.Error(w, `{"error":{"status":400,"name":"ValidationError","message":"Invalid key archetype"}}`, http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer strapi.Close()

	archive{url: strapi.URL, token: "t"}.save(entry{Code: "abcde", Handle: "octocat", Archetype: "The Fork Hoarder"})
	if len(sent) != 2 || sent[0].Archetype == "" || sent[1].Archetype != "" || sent[1].Code != "abcde" {
		t.Errorf("want one try with the archetype and one without, got %+v", sent)
	}
}
