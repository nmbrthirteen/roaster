// Command roaster is the server behind the stand: the kiosk page, the receipt
// designer and the hidden menu. The HTTP surface lives in internal/server; this
// is the settings, the credentials and the listen.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/upgaming/roaster/internal/config"
	"github.com/upgaming/roaster/internal/github"
	"github.com/upgaming/roaster/internal/metric"
	"github.com/upgaming/roaster/internal/receipt"
	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/secret"
	"github.com/upgaming/roaster/internal/server"
	"github.com/upgaming/roaster/internal/state"
	"github.com/upgaming/roaster/internal/ui"
	"github.com/upgaming/roaster/internal/version"
)

func main() {
	configPath := flag.String("config", "roaster.json", "per-device settings file")
	flag.String("addr", ":3000", "listen address")
	flag.String("events", "events", "directory of event files, overriding the built-in ones")
	flag.String("terminal", "001", "terminal number printed on every receipt")
	flag.String("printer", "", "printer: tcp:host:9100, lp:queue or file:path")
	flag.String("event", "", "event code the kiosk opens on")
	facts := flag.String("facts", "", "read one GitHub account, print what the receipt would say, and exit")
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
	if *facts != "" {
		readFacts(*facts)
		return
	}

	state.Seed("roaster.json")
	*configPath = state.Resolve(*configPath)

	// A damaged settings file must not leave a dead stand. It starts on the
	// defaults, and the first setting saved from the hidden menu rewrites it.
	saved, err := config.Load(*configPath)
	if err != nil {
		log.Printf("config: %v (starting on defaults)", err)
		saved = config.Defaults()
	}
	cfg := saved
	config.ApplyFlags(&cfg, flag.CommandLine)

	state.SeedDir(cfg.EventsDir)
	cfg.EventsDir = state.Resolve(cfg.EventsDir)
	saved.EventsDir = state.Resolve(saved.EventsDir)

	log.Printf("roasts: %s", cfg.Provider)

	assets, err := receipt.LoadAssets(ui.FS, "assets")
	if err != nil {
		log.Printf("assets: %v (receipts will print without the logo)", err)
	}

	srv, err := server.New(server.Options{
		Config: cfg,
		Saved:  saved,
		Path:   *configPath,
		Assets: assets,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("roaster listening on %s, terminal #%s", cfg.Addr, srv.Terminal())
	log.Fatal(http.ListenAndServe(cfg.Addr, srv.Handler()))
}

// readFacts is how to see a real audit without a model, a printer or a stand.
// It prints what the gauges would say and how long GitHub took to say it,
// because a queue at a booth is the constraint everything here is built around.
//
// The token is read from the environment and nowhere else. A flag would put it
// in shell history and in the process list of a machine other people use.
func readFacts(handle string) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("set GITHUB_TOKEN first: GitHub's GraphQL API refuses anonymous calls")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	started := time.Now()
	f, err := github.Client{Token: token}.Read(ctx, handle)
	if err != nil {
		log.Fatal(err)
	}
	took := time.Since(started)

	metrics := metric.From(f)
	score := metric.Score(metrics)

	fmt.Printf("\n@%s", f.Handle)
	if f.Name != "" {
		fmt.Printf(", %s", f.Name)
	}
	fmt.Printf("\njoined %s, %d followers, %d repos, %d forks\n",
		f.Created.Format("January 2006"), f.Followers, f.Owned, f.Forked)
	fmt.Printf("read %d commits across %d repos\n\n", len(f.Commits), len(f.Repos))

	for _, m := range metrics {
		fmt.Printf("  %-30s %8s  %s\n", m.Label, m.Value, m.Tag)
	}
	fmt.Printf("\n  %-30s %8s  %s\n", "Score", fmt.Sprintf("%d / 100", score), roast.Severity(score))
	fmt.Printf("\nGitHub answered in %s\n\n", took.Round(time.Millisecond))
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
	log.Printf("--- roaster %s starting, %s/%s ---", version.Version, runtime.GOOS, runtime.GOARCH)
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
