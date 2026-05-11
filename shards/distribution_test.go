package shards

import "testing"

// TestParseDestination covers the surface every HTTP request goes through.
// "" maps to Mixed because the upstream shell drivers occasionally omit
// the field.
func TestParseDestination_Aliases(t *testing.T) {
	cases := map[string]Destination{
		"same":         SameShard,
		"same_shard":   SameShard,
		"cross":        CrossShard,
		"cross_shard":  CrossShard,
		"mixed":        Mixed,
		"":             Mixed,
	}
	for input, want := range cases {
		got, err := ParseDestination(input)
		if err != nil {
			t.Fatalf("ParseDestination(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("ParseDestination(%q): got %d, want %d", input, got, want)
		}
	}
}

func TestParseDestination_UnknownIsRejected(t *testing.T) {
	if _, err := ParseDestination("orbit"); err == nil {
		t.Fatalf("expected error for unknown destination")
	}
}

func TestCoordinator_RejectsZeroShards(t *testing.T) {
	if _, err := NewCoordinator(0); err == nil {
		t.Fatalf("expected error when numShards = 0")
	}
}

func TestCoordinator_NumShardsRoundTrip(t *testing.T) {
	for _, n := range []uint32{1, 2, 3, 4, 8} {
		sc, err := NewCoordinator(n)
		if err != nil {
			t.Fatalf("NewCoordinator(%d): %v", n, err)
		}
		if got := sc.NumShards(); got != n {
			t.Fatalf("NumShards: got %d, want %d", got, n)
		}
	}
}
