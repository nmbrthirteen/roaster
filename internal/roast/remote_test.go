package roast

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPrintIsReportedBesideTheRoastRoute(t *testing.T) {
	var path, auth, code string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		code = body["code"]
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	err := Remote{URL: srv.URL + "/roast?x=1", Token: "tok"}.Printed(context.Background(), "abcde")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/printed" || auth != "Bearer tok" || code != "abcde" {
		t.Errorf("got %s %q %q", path, auth, code)
	}
}

func TestARemoteSearchSaysWhatTheServiceSaid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" || r.Header.Get("Authorization") != "Bearer t" {
			http.Error(w, "wrong", http.StatusBadRequest)
			return
		}
		var s Search
		json.NewDecoder(r.Body).Decode(&s)
		if s.Query == "nobody" {
			json.NewEncoder(w).Encode(map[string]any{"candidates": []Candidate{}, "error": "type a first and last name"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"candidates": []Candidate{{Ref: "jordan-sample", Name: s.Query}}})
	}))
	defer srv.Close()

	r := Remote{URL: srv.URL + "/roast", Token: "t"}
	found, err := r.Search(context.Background(), Search{Query: "Jordan Sample", Pack: LinkedIn})
	if err != nil || len(found) != 1 || found[0].Ref != "jordan-sample" {
		t.Fatalf("got %v %v", found, err)
	}
	if _, err := r.Search(context.Background(), Search{Query: "nobody"}); err == nil || err.Error() != "type a first and last name" {
		t.Errorf("the service's own words should reach the visitor, got %v", err)
	}
}
