package main

import "testing"

func TestForceKiosk(t *testing.T) {
	cases := map[string]string{
		"http://localhost:3000":              "http://localhost:3000/kiosk",
		"http://localhost:3000/":             "http://localhost:3000/kiosk",
		"http://localhost:3000/preview":      "http://localhost:3000/kiosk",
		"http://localhost:3000/kiosk":        "http://localhost:3000/kiosk",
		"http://localhost:3000/preview?a=1":  "http://localhost:3000/kiosk",
		"https://roast.upgaming.com/preview": "https://roast.upgaming.com/kiosk",
	}
	for in, want := range cases {
		if got := forceKiosk(in); got != want {
			t.Errorf("forceKiosk(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}
