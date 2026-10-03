package netconf

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRetryDroppedResendsBody(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Write(body)
	}))
	defer srv.Close()

	client := &http.Client{Transport: retrying{next: &http.Transport{
		DialContext: (&net.Dialer{}).DialContext,
	}}}
	res, err := client.Post(srv.URL, "text/plain", strings.NewReader("roast"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	got, _ := io.ReadAll(res.Body)
	if string(got) != "roast" || calls != 2 {
		t.Fatalf("got %q after %d calls, want %q after 2", got, calls, "roast")
	}
}
