// Package stats records and reports submitted-transaction rates.
//
// The Sampler is a small in-memory ring of timestamped counts. Each call
// to Record(scenario, n) appends an entry and expires anything older than
// the longest configured reporting window. Reporters compute TPS over a
// rolling time window by summing the counts in the ring whose timestamps
// fall within (now - window, now].
//
// "Submitted TPS" reflects what the txgen pushed at the proxy, not what
// the chain included in a block. An honest "included TPS" reading requires
// polling the proxy's /network/status/{shard} endpoint and tracking per-
// shard tx counts — that path is not yet wired here and is the natural
// follow-up. Until then, included-TPS must be read directly off the
// proxy/explorer.
package stats

import (
	"sort"
	"sync"
	"time"
)

// entry is one (timestamp, scenario, count) tuple in the ring.
type entry struct {
	at       time.Time
	scenario string
	count    int
}

// Sampler is concurrency-safe and lock-free for reads after Record returns
// (callers cannot mutate the entry slice).
type Sampler struct {
	mu          sync.Mutex
	entries     []entry
	maxWindow   time.Duration
	clock       func() time.Time // injectable for tests
}

// Window is one named reporting interval, e.g. "1m", "5m". Reporters
// compute TPS over each configured window in a single pass.
type Window struct {
	Name     string
	Duration time.Duration
}

// New constructs a sampler that retains entries for at least maxWindow.
// Entries older than maxWindow are evicted on every Record call so memory
// stays bounded regardless of throughput.
func New(maxWindow time.Duration) *Sampler {
	return &Sampler{
		maxWindow: maxWindow,
		clock:     time.Now,
	}
}

// Record appends one count for a scenario at the current sampler-clock
// time. n=0 is a no-op so handlers do not need a defensive guard.
func (s *Sampler) Record(scenario string, n int) {
	if n <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	s.entries = append(s.entries, entry{at: now, scenario: scenario, count: n})
	s.evictOlderThan(now.Add(-s.maxWindow))
}

// evictOlderThan removes entries with timestamps strictly older than
// cutoff. Entries are appended in chronological order so the first
// in-window index identifies the prefix to drop. Must be called with
// s.mu held.
func (s *Sampler) evictOlderThan(cutoff time.Time) {
	i := sort.Search(len(s.entries), func(i int) bool {
		return s.entries[i].at.After(cutoff) || s.entries[i].at.Equal(cutoff)
	})
	if i > 0 {
		s.entries = s.entries[i:]
	}
}

// Report is the rolled-up TPS view for one window.
type Report struct {
	Window         string             `json:"window"`
	WindowSeconds  float64            `json:"windowSeconds"`
	TotalTxs       int                `json:"totalTxs"`
	OverallTPS     float64            `json:"overallTPS"`
	PerScenarioTxs map[string]int     `json:"perScenarioTxs,omitempty"`
	PerScenarioTPS map[string]float64 `json:"perScenarioTPS,omitempty"`
}

// Report computes TPS over each requested window. Multiple windows are
// computed in a single scan of the ring.
func (s *Sampler) Report(windows []Window) []Report {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	out := make([]Report, 0, len(windows))
	for _, w := range windows {
		cutoff := now.Add(-w.Duration)
		report := Report{
			Window:         w.Name,
			WindowSeconds:  w.Duration.Seconds(),
			PerScenarioTxs: map[string]int{},
			PerScenarioTPS: map[string]float64{},
		}
		for _, e := range s.entries {
			if e.at.Before(cutoff) {
				continue
			}
			report.TotalTxs += e.count
			report.PerScenarioTxs[e.scenario] += e.count
		}
		if w.Duration > 0 {
			report.OverallTPS = float64(report.TotalTxs) / w.Duration.Seconds()
			for k, v := range report.PerScenarioTxs {
				report.PerScenarioTPS[k] = float64(v) / w.Duration.Seconds()
			}
		}
		out = append(out, report)
	}
	return out
}

// Len returns the number of retained entries. Useful for diagnostics and
// for tests that need to assert eviction.
func (s *Sampler) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// setClock overrides the time source. Test-only.
func (s *Sampler) setClock(fn func() time.Time) {
	s.mu.Lock()
	s.clock = fn
	s.mu.Unlock()
}
