package main

import "testing"

func TestAtPinsThePath(t *testing.T) {
	cases := []struct{ in, page, want string }{
		{"http://localhost:3000", "/kiosk", "http://localhost:3000/kiosk"},
		{"http://localhost:3000/", "/kiosk", "http://localhost:3000/kiosk"},
		{"http://localhost:3000/preview", "/kiosk", "http://localhost:3000/kiosk"},
		{"http://localhost:3000/kiosk", "/kiosk", "http://localhost:3000/kiosk"},
		{"http://localhost:3000/kiosk?a=1", "/kiosk", "http://localhost:3000/kiosk"},
		{"http://localhost:3000/kiosk", "/preview", "http://localhost:3000/preview"},
		{"https://roast.upgaming.com", "/preview", "https://roast.upgaming.com/preview"},
	}
	for _, c := range cases {
		if got := at(c.in, c.page); got != c.want {
			t.Errorf("at(%q, %q)\n got %q\nwant %q", c.in, c.page, got, c.want)
		}
	}
}
