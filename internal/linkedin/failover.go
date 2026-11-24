package linkedin

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/upgaming/roaster/internal/roast"
)

// ErrVendors is every vendor failing at once. The visitor gets something they
// can act on; the detail goes to the log.
var ErrVendors = errors.New("LinkedIn profiles cannot be read right now; try again in a moment")

// Vendor is one Source with the name the log and the hidden menu use for it.
type Vendor struct {
	Name string
	Source
}

// Failover asks each vendor in turn, giving each its own deadline, so a vendor
// that hangs costs the queue Wait and no more.
type Failover struct {
	Vendors []Vendor
	Wait    time.Duration // per vendor; zero is eight seconds
}

const defaultWait = 8 * time.Second

func (f Failover) Search(ctx context.Context, name string) ([]roast.Candidate, error) {
	return first(ctx, f, "search", func(ctx context.Context, v Vendor) ([]roast.Candidate, error) {
		return v.Search(ctx, name)
	})
}

func (f Failover) Profile(ctx context.Context, ref string) (Profile, error) {
	return first(ctx, f, "profile", func(ctx context.Context, v Vendor) (Profile, error) {
		return v.Profile(ctx, ref)
	})
}

// first returns the first vendor's answer. A profile one vendor cannot find
// may still be in another's, so ErrNoProfile only stands once every vendor
// has said it.
func first[T any](ctx context.Context, f Failover, call string, ask func(context.Context, Vendor) (T, error)) (T, error) {
	var zero T
	if len(f.Vendors) == 0 {
		return zero, ErrVendors
	}
	missing := 0
	for _, v := range f.Vendors {
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}
		started := time.Now()
		vctx, cancel := context.WithTimeout(ctx, f.wait())
		got, err := ask(vctx, v)
		cancel()
		ms := time.Since(started).Milliseconds()
		if err == nil {
			slog.Info("linkedin", "vendor", v.Name, "call", call, "ms", ms)
			return got, nil
		}
		if errors.Is(err, ErrNoProfile) {
			missing++
		}
		slog.Warn("linkedin", "vendor", v.Name, "call", call, "ms", ms, "err", err)
	}
	if missing == len(f.Vendors) {
		return zero, ErrNoProfile
	}
	return zero, ErrVendors
}

func (f Failover) wait() time.Duration {
	if f.Wait == 0 {
		return defaultWait
	}
	return f.Wait
}
