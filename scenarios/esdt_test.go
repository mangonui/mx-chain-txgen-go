package scenarios

import (
	"encoding/hex"
	"testing"

	coreTxn "github.com/multiversx/mx-chain-core-go/data/transaction"
	sdkData "github.com/multiversx/mx-sdk-go/data"
)

// canonicalESDTSystemSCHex is the well-known 32-byte raw address of the
// ESDT issuance system smart contract. Sourced from mx-chain-go and
// mirrored in mx-sdk-py. This is the source-of-truth bytes; the bech32
// constant in esdt.go is derived from these.
const canonicalESDTSystemSCHex = "000000000000000000010000000000000000000000000000000000000002ffff"

// TestESDTSystemSCAddress_MatchesCanonicalHex is a regression guard
// against the same class of typo that caused the v0 bech32 constant
// ("erd1qqqqqqqqqqqqqqqpqqqqqqqqlllllls8a5w6u" — 40 chars, missing 22
// 'q's in the middle) to silently land in the repo and only surface
// when the live chain rejected every ESDT issue with HTTP 500.
//
// The test decodes the constant the scenario actually uses and asserts
// its raw bytes equal the canonical hex. A future edit that breaks the
// constant in any way (wrong padding, wrong byte order, off-by-N
// position) fails this test before reaching a chain.
func TestESDTSystemSCAddress_MatchesCanonicalHex(t *testing.T) {
	got, err := sdkData.NewAddressFromBech32String(esdtSystemSCAddress)
	if err != nil {
		t.Fatalf("decode constant %q: %v", esdtSystemSCAddress, err)
	}
	if len(got.AddressBytes()) != 32 {
		t.Fatalf("address bytes len: got %d, want 32 (MultiversX addresses are 32 bytes)",
			len(got.AddressBytes()))
	}
	gotHex := hex.EncodeToString(got.AddressBytes())
	if gotHex != canonicalESDTSystemSCHex {
		t.Fatalf("ESDT system SC bytes mismatch:\n  got:  %s\n  want: %s\n  (constant bech32 in esdt.go is malformed; see commit history for the historical 40-char typo)",
			gotHex, canonicalESDTSystemSCHex)
	}
}

// makeTxInfoWithLogs synthesises a TransactionInfo with the supplied
// events under .Data.Transaction.Logs.Events. Used to exercise
// extractTokenIdentifier without a live chain.
func makeTxInfoWithLogs(events ...*coreTxn.Events) *sdkData.TransactionInfo {
	info := &sdkData.TransactionInfo{}
	info.Data.Transaction.Logs = &coreTxn.ApiLogs{
		Address: "erd1...issuer...",
		Events:  events,
	}
	return info
}

func TestExtractTokenIdentifier_IssueFungible(t *testing.T) {
	info := makeTxInfoWithLogs(&coreTxn.Events{
		Identifier: "issue",
		Topics:     [][]byte{[]byte("WRK-abc123")},
	})
	got, err := extractTokenIdentifier(info)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if got != "WRK-abc123" {
		t.Fatalf("got %q, want %q", got, "WRK-abc123")
	}
}

func TestExtractTokenIdentifier_IssueSemiFungible(t *testing.T) {
	info := makeTxInfoWithLogs(&coreTxn.Events{
		Identifier: "issueSemiFungible",
		Topics:     [][]byte{[]byte("SFT-deadbe")},
	})
	got, err := extractTokenIdentifier(info)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if got != "SFT-deadbe" {
		t.Fatalf("got %q, want %q", got, "SFT-deadbe")
	}
}

func TestExtractTokenIdentifier_IssueNonFungible(t *testing.T) {
	info := makeTxInfoWithLogs(&coreTxn.Events{
		Identifier: "issueNonFungible",
		Topics:     [][]byte{[]byte("NFT-feedf0")},
	})
	got, err := extractTokenIdentifier(info)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if got != "NFT-feedf0" {
		t.Fatalf("got %q, want %q", got, "NFT-feedf0")
	}
}

func TestExtractTokenIdentifier_SkipsUnrelatedEvents(t *testing.T) {
	info := makeTxInfoWithLogs(
		&coreTxn.Events{
			Identifier: "transferValueOnly",
			Topics:     [][]byte{[]byte("not-a-token-id")},
		},
		&coreTxn.Events{
			Identifier: "issue",
			Topics:     [][]byte{[]byte("WRK-abc123")},
		},
		&coreTxn.Events{
			Identifier: "completedTxEvent",
			Topics:     [][]byte{},
		},
	)
	got, err := extractTokenIdentifier(info)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if got != "WRK-abc123" {
		t.Fatalf("got %q, want %q (extractor must skip non-issuance events)", got, "WRK-abc123")
	}
}

func TestExtractTokenIdentifier_NilInfoIsError(t *testing.T) {
	if _, err := extractTokenIdentifier(nil); err == nil {
		t.Fatalf("expected error for nil info")
	}
}

func TestExtractTokenIdentifier_NoLogsIsError(t *testing.T) {
	info := &sdkData.TransactionInfo{} // Logs is nil
	if _, err := extractTokenIdentifier(info); err == nil {
		t.Fatalf("expected error when Logs is nil")
	}
}

func TestExtractTokenIdentifier_NoIssuanceEventIsError(t *testing.T) {
	info := makeTxInfoWithLogs(&coreTxn.Events{
		Identifier: "transferValueOnly",
		Topics:     [][]byte{[]byte("nope")},
	})
	if _, err := extractTokenIdentifier(info); err == nil {
		t.Fatalf("expected error when no issuance event is present")
	}
}

func TestExtractTokenIdentifier_EmptyTopicSkipped(t *testing.T) {
	info := makeTxInfoWithLogs(
		&coreTxn.Events{
			Identifier: "issue",
			Topics:     [][]byte{}, // malformed — no token id
		},
		&coreTxn.Events{
			Identifier: "issue",
			Topics:     [][]byte{[]byte("WRK-abc123")},
		},
	)
	got, err := extractTokenIdentifier(info)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if got != "WRK-abc123" {
		t.Fatalf("got %q, want %q (extractor must skip empty-topic events)", got, "WRK-abc123")
	}
}

func TestExtractTokenIdentifier_NilEventSkipped(t *testing.T) {
	info := makeTxInfoWithLogs(
		nil,
		&coreTxn.Events{
			Identifier: "issue",
			Topics:     [][]byte{[]byte("WRK-abc123")},
		},
	)
	got, err := extractTokenIdentifier(info)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if got != "WRK-abc123" {
		t.Fatalf("got %q, want %q", got, "WRK-abc123")
	}
}
