// Package linkedin roasts a LinkedIn profile. A visitor types their name, picks
// themselves from the people it matches, and the profile is read, measured and
// roasted the way a GitHub account is.
//
// LinkedIn has no official API that finds a member by name or reads another
// member's profile, so the profile comes from a data vendor behind Source. Two
// vendors in a Failover keep one outage from emptying the stand.
package linkedin

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/upgaming/roaster/internal/roast"
)

var (
	// ErrNoProfile is a profile that is not there, or not public.
	ErrNoProfile = errors.New("no public LinkedIn profile there")

	// ErrBadRef is a profile reference no LinkedIn URL can hold.
	ErrBadRef = errors.New("is not a LinkedIn profile")
)

// Source is where profiles come from: a vendor, or the rehearsal set.
type Source interface {
	// Search returns the public profiles a name matches, best first.
	Search(ctx context.Context, name string) ([]roast.Candidate, error)
	// Profile reads one profile by its public reference, the part of the URL
	// after linkedin.com/in/.
	Profile(ctx context.Context, ref string) (Profile, error)
}

type Profile struct {
	Ref       string // linkedin.com/in/<Ref>
	Name      string
	Headline  string
	About     string
	Location  string
	Photo     string
	Followers int

	Positions      []Position // newest first
	Education      []School
	Skills         []string
	Certifications []string
	Languages      []string
	Posts          []Post // the latest few; a vendor without posts leaves it empty
}

type Position struct {
	Title       string
	Company     string
	Description string
	Start       time.Time
	End         time.Time // zero is a role held now
}

func (p Position) Current() bool { return p.End.IsZero() }

type School struct {
	Name   string
	Degree string
	End    time.Time
}

type Post struct {
	Text      string
	At        time.Time
	Reactions int
	Comments  int
}

var ref = regexp.MustCompile(`^[\p{L}\p{N}_-]{3,100}$`)

// Valid is a reference LinkedIn could have issued. Vendors are paid per call,
// so anything else is refused before it costs anything.
func Valid(r string) bool {
	return ref.MatchString(r)
}

// Ref pulls the reference out of whatever a visitor or a vendor gives: a full
// profile URL, one without a scheme, or the reference alone. A browser copies
// an accented reference percent-encoded and a vendor returns it decoded, so it
// is always decoded here, and an opt-out copied from either form matches both.
func Ref(s string) string {
	s = strings.TrimSpace(s)
	if _, rest, ok := strings.Cut(s, "linkedin.com/in/"); ok {
		s = rest
	}
	s, _, _ = strings.Cut(s, "?")
	s, _, _ = strings.Cut(s, "#")
	s = strings.Trim(s, "/")
	if decoded, err := url.PathUnescape(s); err == nil {
		s = decoded
	}
	return s
}
