// Command roastd is the roast service, and the only place the model key and the
// GitHub token exist. Stands post a handle to it with their terminal token and
// get the audit back as a stream of server-sent events, which the kiosk's
// "remote" provider already speaks.
//
// It holds nothing that cannot be lost: the cache is a convenience, and a
// restart costs a few repeat lookups. So it scales by running more copies of
// it behind a load balancer, and it deploys anywhere that sets PORT.
//
// Configuration is the environment alone. See .env.example.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/upgaming/roaster/internal/audit"
	"github.com/upgaming/roaster/internal/github"
	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/verdict"
)

// A terminal token shorter than this is a password someone typed, not a token
// someone generated.
const minTokenLength = 24

type settings struct {
	addr      string
	github    string
	writer    string // "openai", "anthropic", or empty for the numbers alone
	openAI    string // OPENAI_API_KEY
	model     string // OPENAI_MODEL
	xAI       string // XAI_API_KEY
	grokModel string // XAI_MODEL
	tokens    []string
	optOut    map[string]bool
	parallel  int

	strapiURL   string // STRAPI_URL; with STRAPI_TOKEN, every roast is saved there
	strapiToken string // STRAPI_TOKEN, allowed to create roast entries and nothing more
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := fromEnv(os.Getenv)
	if err != nil {
		slog.Error("configuration", "err", err)
		os.Exit(1)
	}

	provider := audit.Audit{
		GitHub:  github.Client{Token: cfg.github},
		Blocked: cfg.optOut,
		// Ten minutes covers a crowd passing one handle around, and a
		// thousand briefs is a few megabytes.
		Memory: audit.NewMemory(10*time.Minute, 1000),
	}
	writers := map[string]verdict.Writer{}
	if cfg.openAI != "" {
		writers["openai-terra"] = verdict.OpenAI{Key: cfg.openAI, Model: cfg.model}
		writers["openai-luna"] = verdict.OpenAI{Key: cfg.openAI, Model: "gpt-6-luna"}
		// Existing saved cheap selections move to GPT-6 Luna.
		writers["openai-mini"] = writers["openai-luna"]
	}
	if cfg.xAI != "" {
		writers["grok"] = verdict.OpenAI{Key: cfg.xAI, Model: cfg.grokModel, URL: "https://api.x.ai/v1/responses", Provider: "xAI"}
	}
	if cfg.writer == "anthropic" {
		writers["anthropic"] = verdict.NewClaude()
	}
	if len(writers) == 0 {
		slog.Warn("no model key is set; verdicts will be written from the numbers alone")
	}

	var router roast.Provider = roast.Router{roast.GitHub: modelRouter{audit: provider, writers: writers, fallback: defaultModel(cfg)}}
	if cfg.strapiURL != "" && cfg.strapiToken != "" {
		router = archive{next: router, url: cfg.strapiURL, token: cfg.strapiToken}
	} else {
		slog.Warn("STRAPI_URL or STRAPI_TOKEN is not set; roasts will not be saved and share links will not open")
	}

	svc := newService(router, cfg.tokens, limits{
		slots:  cfg.parallel,
		queue:  10 * time.Second,
		budget: 40 * time.Second,
		every:  3 * time.Second,
		burst:  6,
	})

	srv := &http.Server{
		Addr:    cfg.addr,
		Handler: svc.handler(),
		// Headers have to arrive promptly. There is no write timeout, because
		// an audit is a stream that runs as long as the audit does, and the
		// budget above is what bounds it.
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		slog.Info("listening", "addr", cfg.addr, "terminals", len(cfg.tokens), "slots", cfg.parallel, "models", len(writers))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	<-stop.Done()

	// A deploy should not cut anyone off mid-roast. Stop taking new ones and
	// let the running ones finish, up to the length of one audit.
	slog.Info("shutting down; letting running roasts finish")
	ctx, done := context.WithTimeout(context.Background(), 45*time.Second)
	defer done()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown", "err", err)
	}
}

// fromEnv reads the settings, refusing to start on anything that would make the
// service insecure or useless rather than discovering it on the first request.
func fromEnv(env func(string) string) (settings, error) {
	cfg := settings{
		addr:      ":8080",
		github:    env("GITHUB_TOKEN"),
		openAI:    env("OPENAI_API_KEY"),
		model:     env("OPENAI_MODEL"),
		xAI:       env("XAI_API_KEY"),
		grokModel: env("XAI_MODEL"),
		optOut:    map[string]bool{},
		parallel:  16,

		strapiURL:   env("STRAPI_URL"),
		strapiToken: env("STRAPI_TOKEN"),
	}
	if cfg.grokModel == "" {
		cfg.grokModel = "grok-4.7"
	}
	// OpenAI when its key is there, Claude when only that one is, and the
	// numbers alone when neither is. One writer, chosen once, logged at start.
	switch {
	case cfg.openAI != "":
		cfg.writer = "openai"
	case cfg.xAI != "":
		cfg.writer = "grok"
	case env("ANTHROPIC_API_KEY") != "":
		cfg.writer = "anthropic"
	}
	if port := env("PORT"); port != "" {
		cfg.addr = ":" + port
	}

	if cfg.github == "" {
		return cfg, fmt.Errorf("GITHUB_TOKEN is required; GitHub's GraphQL API refuses anonymous calls")
	}

	for _, t := range strings.Split(env("TERMINAL_TOKENS"), ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if len(t) < minTokenLength {
			return cfg, fmt.Errorf("a terminal token is shorter than %d characters; generate them, do not type them", minTokenLength)
		}
		cfg.tokens = append(cfg.tokens, t)
	}
	if len(cfg.tokens) == 0 {
		return cfg, fmt.Errorf("TERMINAL_TOKENS is required: with none, no stand could reach the service")
	}

	for _, h := range strings.Split(env("ROAST_OPT_OUT"), ",") {
		if h = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(h), "@")); h != "" {
			cfg.optOut[h] = true
		}
	}

	if v := env("ROAST_PARALLEL"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return cfg, fmt.Errorf("ROAST_PARALLEL should be a whole number of 1 or more, got %q", v)
		}
		cfg.parallel = n
	}
	return cfg, nil
}

func defaultModel(cfg settings) string {
	switch cfg.writer {
	case "openai":
		return "openai-terra"
	case "grok":
		return "grok"
	case "anthropic":
		return "anthropic"
	default:
		return ""
	}
}
