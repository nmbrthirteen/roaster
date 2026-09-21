package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/secret"
)

// errNotSetUp is what a visitor sees when the stand cannot roast yet. The
// detail goes to the hidden menu, where it can be fixed.
var errNotSetUp = errors.New("this stand is not set up to roast yet. Ask anyone at the stand")

// source builds the roast provider from the settings as they are now, so a
// fix made in the hidden menu applies to the very next visitor.
func (s *Server) source() (roast.Provider, error) {
	if s.provider != nil {
		return s.provider, nil
	}
	cfg := s.st.config()
	switch cfg.Provider {
	case "", "demo":
		return roast.Demo{}, nil
	case "remote":
		if cfg.RemoteURL == "" {
			return nil, fmt.Errorf("No roast service address yet. Enter it under Roast service.")
		}
		token, err := secret.Load()
		if err != nil {
			return nil, fmt.Errorf("The terminal token cannot be read (%v). Paste it again under Roast service.", err)
		}
		if token == "" {
			return nil, fmt.Errorf("No terminal token yet. Paste it under Roast service.")
		}
		return roast.Remote{URL: cfg.RemoteURL, Token: token}, nil
	}
	return nil, fmt.Errorf("Unknown roast source %q. Pick one under Roast service.", cfg.Provider)
}

// checkService asks the roast service's health route, beside the roast route,
// whether it is up. It proves the address and the network, not the token.
func (s *Server) checkService(ctx context.Context) (string, error) {
	if _, err := s.source(); err != nil {
		return "", err
	}
	cfg := s.st.config()
	if cfg.Provider != "remote" {
		return "Demo mode: roasts are rehearsal data and nothing is contacted.", nil
	}
	u, err := url.Parse(cfg.RemoteURL)
	if err != nil {
		return "", err
	}
	u.Path, u.RawQuery = "/health", ""

	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	started := time.Now()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Could not reach %s. Check the Wi-Fi and the address. (%v)", u.Host, err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s answered %s. Check the address.", u.Host, res.Status)
	}
	return fmt.Sprintf("Roast service answered in %d ms. The token is checked on the first roast.",
		time.Since(started).Milliseconds()), nil
}
