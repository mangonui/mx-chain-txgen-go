package shards

import (
	"strings"
	"testing"
)

// deployerPubKey32 is a fixed 32-byte vector to keep the derivation tests
// deterministic. The exact bytes are irrelevant — what matters is that the
// same input always produces the same address (deterministic) and that
// different inputs produce different addresses (sensitivity).
var deployerPubKey32 = []byte{
	0xd6, 0xe8, 0xb7, 0xfa, 0x1e, 0x71, 0x14, 0x31,
	0xe6, 0x15, 0xe1, 0x6c, 0x13, 0x51, 0x58, 0xa0,
	0x28, 0xc9, 0x70, 0x8e, 0x00, 0x8a, 0x9c, 0x35,
	0x45, 0x59, 0x81, 0x67, 0x1d, 0xbd, 0x42, 0x52,
}

func TestComputeContractAddress_Deterministic(t *testing.T) {
	a, err := ComputeContractAddress(deployerPubKey32, 0, 2)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	b, err := ComputeContractAddress(deployerPubKey32, 0, 2)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if a != b {
		t.Fatalf("non-deterministic: a=%s b=%s", a, b)
	}
}

func TestComputeContractAddress_NonceSensitivity(t *testing.T) {
	a0, err := ComputeContractAddress(deployerPubKey32, 0, 2)
	if err != nil {
		t.Fatalf("nonce 0: %v", err)
	}
	a1, err := ComputeContractAddress(deployerPubKey32, 1, 2)
	if err != nil {
		t.Fatalf("nonce 1: %v", err)
	}
	if a0 == a1 {
		t.Fatalf("nonce should change address: %s", a0)
	}
}

func TestComputeContractAddress_LooksLikeSCAddress(t *testing.T) {
	addr, err := ComputeContractAddress(deployerPubKey32, 0, 2)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	// Contract addresses in MultiversX have a known structural prefix:
	// the first ten bytes (post-VM-type-pad) are zero, which in bech32
	// renders as a long run of leading 'q's. Any production tooling
	// asserting "is contract" looks for this. We only enforce that the
	// address starts with "erd1qqqqqq" which is the floor.
	if !strings.HasPrefix(addr, "erd1qqqqqq") {
		t.Fatalf("address %s does not look like a contract address", addr)
	}
}

func TestComputeContractAddress_RejectsEmptyPubKey(t *testing.T) {
	if _, err := ComputeContractAddress(nil, 0, 2); err == nil {
		t.Fatalf("expected error for empty pubkey")
	}
	if _, err := ComputeContractAddress([]byte{}, 0, 2); err == nil {
		t.Fatalf("expected error for empty pubkey")
	}
}

func TestComputeContractAddress_RejectsZeroShards(t *testing.T) {
	if _, err := ComputeContractAddress(deployerPubKey32, 0, 0); err == nil {
		t.Fatalf("expected error for zero shards")
	}
}
