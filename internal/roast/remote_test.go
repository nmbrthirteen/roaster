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
