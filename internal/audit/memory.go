package audit

import (
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/upgaming/roaster/internal/roast"
	"github.com/upgaming/roaster/internal/verdict"
)

const (
	// Lines kept across everyone. Enough to cover the stretch of queue that
	// can read each other's receipts.
	keepRecent = 12

	// Lines kept per account. Enough that someone roasting themselves again
	// and again keeps getting something new.
	keepSaid = 3
)

// Memory is what the audit keeps between visitors, and it splits the work on
// purpose. The account is read, measured and briefed once, because that part
// is slow, costs GitHub budget and does not change from one minute to the next.
// The verdict is written fresh every time, because that is the part people
// compare, and the same joke twice is no joke at all.
type Memory struct {
	mu       sync.Mutex
	ttl      time.Duration
	max      int
	accounts map[string]*known
	recent   []string // newest last
	flight   singleflight.Group
	now      func() time.Time
}

type known struct {
	measured
	said    []string // lines printed to this account, newest last
	expires time.Time
}

type measured struct {
	handle string // as GitHub spells it
	brief  verdict.Brief

	story     []roast.Section
	actions   []string
	strengths []string
	findings  []roast.Finding
	habits    []roast.Finding
	feed      []roast.Item
	exhibit   *roast.Item
	heat      [][7]int
}

// NewMemory keeps an account for ttl, and at most max of them. A brief is a few
// kilobytes, so the cap is about memory, not about how busy a stand gets.
func NewMemory(ttl time.Duration, max int) *Memory {
	return &Memory{ttl: ttl, max: max, accounts: map[string]*known{}, now: time.Now}
}

// measure returns the account read and briefed: from memory while it is fresh,
// otherwise from read, run once however many stands ask at the same moment. A
// failure is never kept, so the next attempt tries again.
func (m *Memory) measure(key string, read func() (measured, error)) (measured, error) {
	m.mu.Lock()
	if k, ok := m.accounts[key]; ok && m.now().Before(k.expires) {
		m.mu.Unlock()
		return k.measured, nil
	}
	m.mu.Unlock()

	v, err, _ := m.flight.Do(key, func() (any, error) {
		got, err := read()
		if err != nil {
			return nil, err
		}
		m.keep(key, got)
		return got, nil
	})
	if err != nil {
		return measured{}, err
	}
	return v.(measured), nil
}

func (m *Memory) keep(key string, got measured) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	if len(m.accounts) >= m.max {
		for k, v := range m.accounts {
			if now.After(v.expires) {
				delete(m.accounts, k)
			}
		}
	}
	// Still full of live accounts: drop arbitrary ones. Go's map order is
	// random, which is the eviction this needs, for free.
	for k := range m.accounts {
		if len(m.accounts) < m.max {
			break
		}
		delete(m.accounts, k)
	}

	var said []string
	if old, ok := m.accounts[key]; ok {
		said = old.said // an expired read keeps what was already said to them
	}
	m.accounts[key] = &known{measured: got, said: said, expires: now.Add(m.ttl)}
}

// avoid is what the next verdict for this account must not repeat: what this
// account was told before, then what the queue has read lately.
func (m *Memory) avoid(key string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	seen := map[string]bool{}
	var out []string
	add := func(lines []string) {
		for i := len(lines) - 1; i >= 0; i-- {
			if l := lines[i]; !seen[l] {
				seen[l] = true
				out = append(out, l)
			}
		}
	}
	if k, ok := m.accounts[key]; ok {
		add(k.said)
	}
	add(m.recent)
	return out
}

// said records a line that was printed.
func (m *Memory) said(key, line string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.recent = last(append(m.recent, line), keepRecent)
	if k, ok := m.accounts[key]; ok {
		k.said = last(append(k.said, line), keepSaid)
	}
}

func last(lines []string, n int) []string {
	if len(lines) <= n {
		return lines
	}
	return append([]string(nil), lines[len(lines)-n:]...)
}

// key folds case, because GitHub does: @Torvalds and @torvalds are one person.
func key(handle string) string {
	return strings.ToLower(handle)
}
