package submit

import (
	"testing"

	"github.com/mangonui/mx-chain-txgen-go/proxy"
)

func TestNewShouldApplyDefaultBunchSize(t *testing.T) {
	submitter, err := New(&proxy.Client{}, 0)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if submitter == nil {
		t.Fatal("expected submitter")
	}
	if submitter.bunchSize != DefaultBunchSize {
		t.Fatalf("expected bunch size %d, got %d", DefaultBunchSize, submitter.bunchSize)
	}
}

func TestSignAndSubmitWithNoJobsShouldReturnNoHashes(t *testing.T) {
	submitter, err := New(&proxy.Client{}, 10)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	hashes, err := submitter.SignAndSubmit(nil, nil)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if hashes != nil {
		t.Fatalf("expected nil hashes, got %v", hashes)
	}
}
