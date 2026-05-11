package scenarios

import (
	"testing"

	coreTxn "github.com/multiversx/mx-chain-core-go/data/transaction"
	sdkData "github.com/multiversx/mx-sdk-go/data"
)

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
