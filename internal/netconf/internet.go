package netconf

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// Hotel and venue Wi-Fi often joins fine and then answers every request with
// its own sign-in page until someone accepts the terms. Windows finds that
// page with this probe, and so does the stand. It is plain HTTP on purpose:
// a portal can rewrite that, and cannot rewrite HTTPS.
const (
	probeURL  = "http://www.msftconnecttest.com/connecttest.txt"
	probeBody = "Microsoft Connect Test"
)

// probeURLFor is where the probe goes. A test points it at a local server.
var probeURLFor = probeURL

// Internet is what the network does with a request to the outside.
type Internet struct {
	State  string `json:"state"`            // "online", "signin" or "offline"
	Portal string `json:"portal,omitempty"` // the sign-in page, when State is "signin"
}

func CheckInternet(ctx context.Context) Internet {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURLFor, nil)
	if err != nil {
		return Internet{State: "offline"}
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := client.Do(req)
	if err != nil {
		return Internet{State: "offline"}
	}
	defer res.Body.Close()

	if loc, err := res.Location(); err == nil {
		return Internet{State: "signin", Portal: loc.String()}
	} else if !errors.Is(err, http.ErrNoLocation) {
		return Internet{State: "signin", Portal: probeURL}
	}
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
	if res.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == probeBody {
		return Internet{State: "online"}
	}
	// The portal answered in place of the probe. Opening the probe address
	// shows that same page.
	return Internet{State: "signin", Portal: probeURL}
}
