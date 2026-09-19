// Command roaster is the server behind the stand: the kiosk page, the receipt
// designer and the hidden menu. The HTTP surface lives in internal/server; this
// is the settings, the credentials and the listen.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/upgaming/roaster/internal/config"
	"github.com/upgaming/roaster/internal/receipt"
	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/secret"
	"github.com/upgaming/roaster/internal/server"
	"github.com/upgaming/roaster/internal/state"
	"github.com/upgaming/roaster/internal/ui"
)

func main() {
	configPath := flag.String("config", "roaster.json", "per-device settings file")
	flag.String("addr", ":3000", "listen address")
	flag.String("events", "events", "directory of event files, overriding the built-in ones")
	flag.String("terminal", "001", "terminal number printed on every receipt")
	flag.String("printer", "", "printer: tcp:host:9100, lp:queue or file:path")
	flag.String("event", "", "event code the kiosk opens on")
	setToken := flag.Bool("set-token", false, "store this terminal's token, read from standard input")
	stateDir := flag.String("state", "", "settings and token folder, for writing into a packaged install's")
	flag.Parse()

	// Before anything asks where the folder is.
	state.Use(*stateDir)
	startLog()

	if *setToken {
		storeToken()
		return
	}

	state.Seed("roaster.json")
	*configPath = state.Resolve(*configPath)

	saved, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	cfg := saved
	config.ApplyFlags(&cfg, flag.CommandLine)

	state.SeedDir(cfg.EventsDir)
	cfg.EventsDir = state.Resolve(cfg.EventsDir)
	saved.EventsDir = state.Resolve(saved.EventsDir)

	provider, err := pickProvider(cfg)
	if err != nil {
		log.Fatalf("provider: %v", err)
	}
	log.Printf("roasts: %s", cfg.Provider)

	assets, err := receipt.LoadAssets(ui.FS, "assets")
	if err != nil {
		log.Printf("assets: %v (receipts will print without the logo)", err)
	}

	srv, err := server.New(server.Options{
		Config:   cfg,
		Saved:    saved,
		Path:     *configPath,
		Provider: provider,
		Assets:   assets,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("roaster listening on %s, terminal #%s", cfg.Addr, srv.Terminal())
	log.Fatal(http.ListenAndServe(cfg.Addr, srv.Handler()))
}

func pickProvider(cfg config.Config) (roast.Provider, error) {
	switch cfg.Provider {
	case "", "demo":
		return roast.Demo{}, nil
	case "remote":
		if cfg.RemoteURL == "" {
			return nil, fmt.Errorf("provider is remote but remoteUrl is empty")
		}
		token, err := secret.Load()
		if err != nil {
			return nil, err
		}
		if token == "" {
			return nil, fmt.Errorf("no terminal token stored; run roaster -set-token")
		}
		return roast.Remote{URL: cfg.RemoteURL, Token: token}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q, want demo or remote", cfg.Provider)
	}
}

// startLog mirrors the log to a file in the folder the app writes to, and says
// which folder that is. On a packaged install it is not the one the binaries
// are in, and that is the first thing anyone looking for a settings file or a
// log needs to know.
func startLog() {
	path := state.Path("roaster.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("could not open %s: %v", path, err)
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.Printf("--- roaster starting, %s/%s ---", runtime.GOOS, runtime.GOARCH)
	log.Printf("state: %s", state.Dir())
}

// storeToken keeps the token off the command line, where it would land in
// shell history and in the process list.
func storeToken() {
	fmt.Print("Terminal token: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		log.Fatalf("could not read the token: %v", err)
	}
	if err := secret.Store(line); err != nil {
		log.Fatalf("could not store the token: %v", err)
	}
	fmt.Println("Stored. It is encrypted to this machine and is not in any settings file.")
}
