package netconf

import (
	"net/http"
	"time"
)

// RetryDropped makes every client on the default transport send a request
// once more when the connection dies before any response comes back. Venue
// Wi-Fi drops connections often enough that one try fails a visitor every few
// roasts, and the error it leaves is a bare EOF. A response of any status,
// a rate limit included, is returned as is, so this never hammers an API.
func RetryDropped() {
	http.DefaultTransport = retrying{next: http.DefaultTransport}
}

type retrying struct{ next http.RoundTripper }

func (t retrying) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := t.next.RoundTrip(req)
	if err == nil || req.Context().Err() != nil {
		return res, err
	}
	next, ok := again(req)
	if !ok {
		return nil, err
	}
	select {
	case <-time.After(time.Second):
	case <-req.Context().Done():
		return nil, err
	}
	return t.next.RoundTrip(next)
}

// again is req with a fresh body, since the first try consumed it.
func again(req *http.Request) (*http.Request, bool) {
	if req.Body == nil || req.Body == http.NoBody {
		return req, true
	}
	if req.GetBody == nil {
		return nil, false
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, false
	}
	next := req.Clone(req.Context())
	next.Body = body
	return next, true
}
