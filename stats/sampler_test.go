package stats

import (
	"testing"
	"time"
)

// fakeClock returns the same time forever until advanced.
type fakeClock struct{ t time.Time }

func (f *fakeClock) now() time.Time          { return f.t }
func (f *fakeClock) advance(d time.Duration) { f.t = f.t.Add(d) }

func TestSampler_RecordAndReportInsideWindow(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	s := New(5 * time.Minute)
	s.setClock(clk.now)

	s.Record("basic", 100)
	clk.advance(10 * time.Second)
	s.Record("basic", 150)
	clk.advance(20 * time.Second)

	rep := s.Report([]Window{{Name: "1m", Duration: time.Minute}})
	if len(rep) != 1 {
		t.Fatalf("expected one report, got %d", len(rep))
	}
	if rep[0].TotalTxs != 250 {
		t.Fatalf("TotalTxs: got %d, want 250", rep[0].TotalTxs)
	}
	wantTPS := 250.0 / 60.0
	if rep[0].OverallTPS < wantTPS-1e-9 || rep[0].OverallTPS > wantTPS+1e-9 {
		t.Fatalf("OverallTPS: got %v, want %v", rep[0].OverallTPS, wantTPS)
	}
	if rep[0].PerScenarioTxs["basic"] != 250 {
		t.Fatalf("PerScenarioTxs[basic]: got %d, want 250", rep[0].PerScenarioTxs["basic"])
	}
}

func TestSampler_EvictsOutsideMaxWindow(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	s := New(time.Minute)
	s.setClock(clk.now)

	s.Record("basic", 10)
	// Advance past the maxWindow, then record again — the first entry must
	// be evicted as a side-effect of the second Record.
	clk.advance(2 * time.Minute)
	s.Record("basic", 5)

	if got := s.Len(); got != 1 {
		t.Fatalf("Len after eviction: got %d, want 1", got)
	}
}

func TestSampler_MultipleScenarios(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	s := New(time.Minute)
	s.setClock(clk.now)

	s.Record("basic", 100)
	s.Record("erc20", 50)
	s.Record("esdt", 25)

	rep := s.Report([]Window{{Name: "1m", Duration: time.Minute}})
	r := rep[0]
	if r.TotalTxs != 175 {
		t.Fatalf("TotalTxs: got %d, want 175", r.TotalTxs)
	}
	if r.PerScenarioTxs["basic"] != 100 ||
		r.PerScenarioTxs["erc20"] != 50 ||
		r.PerScenarioTxs["esdt"] != 25 {
		t.Fatalf("per-scenario breakdown: %+v", r.PerScenarioTxs)
	}
}

func TestSampler_MultipleWindowsSinglePass(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	s := New(10 * time.Minute)
	s.setClock(clk.now)

	// Old entry (8m ago) — visible to 5m window? no. visible to 10m? yes.
	s.Record("basic", 1000)
	clk.advance(8 * time.Minute)

	// Recent entry (just now) — visible to both windows.
	s.Record("basic", 60)

	rep := s.Report([]Window{
		{Name: "1m", Duration: time.Minute},
		{Name: "5m", Duration: 5 * time.Minute},
		{Name: "10m", Duration: 10 * time.Minute},
	})
	if rep[0].TotalTxs != 60 {
		t.Fatalf("1m: got %d, want 60", rep[0].TotalTxs)
	}
	if rep[1].TotalTxs != 60 {
		t.Fatalf("5m: got %d, want 60 (the 8m-old entry should be excluded)", rep[1].TotalTxs)
	}
	if rep[2].TotalTxs != 1060 {
		t.Fatalf("10m: got %d, want 1060", rep[2].TotalTxs)
	}
}

func TestSampler_ZeroAndNegativeAreNoOps(t *testing.T) {
	s := New(time.Minute)
	s.Record("basic", 0)
	s.Record("basic", -5)
	if got := s.Len(); got != 0 {
		t.Fatalf("Len: got %d, want 0", got)
	}
}

func TestSampler_EmptyReportIsZeroTPS(t *testing.T) {
	s := New(time.Minute)
	rep := s.Report([]Window{{Name: "1m", Duration: time.Minute}})
	if rep[0].TotalTxs != 0 || rep[0].OverallTPS != 0 {
		t.Fatalf("empty: got %+v", rep[0])
	}
}
