package api

import (
	"encoding/json"
	"testing"
)

func TestFlexibleAmount_UnmarshalString(t *testing.T) {
	var a FlexibleAmount
	if err := json.Unmarshal([]byte(`"1000000000000000000"`), &a); err != nil {
		t.Fatalf("unmarshal string: %v", err)
	}
	if a.String() != "1000000000000000000" {
		t.Fatalf("got %q, want %q", a, "1000000000000000000")
	}
}

func TestFlexibleAmount_UnmarshalNumber(t *testing.T) {
	var a FlexibleAmount
	if err := json.Unmarshal([]byte(`1`), &a); err != nil {
		t.Fatalf("unmarshal number: %v", err)
	}
	if a.String() != "1" {
		t.Fatalf("got %q, want %q", a, "1")
	}
}

func TestFlexibleAmount_UnmarshalNumberWithTrailingZero(t *testing.T) {
	// JSON numbers can carry a fractional zero (1.0). The upstream shell
	// drivers never emit this form but a JSON-strict client might.
	var a FlexibleAmount
	if err := json.Unmarshal([]byte(`1.0`), &a); err != nil {
		t.Fatalf("unmarshal 1.0: %v", err)
	}
	if a.String() != "1" {
		t.Fatalf("got %q, want %q", a, "1")
	}
}

func TestFlexibleAmount_UnmarshalNegativeRejected(t *testing.T) {
	var a FlexibleAmount
	if err := json.Unmarshal([]byte(`-1`), &a); err == nil {
		t.Fatalf("expected error for negative value")
	}
}

func TestFlexibleAmount_UnmarshalFractionalRejected(t *testing.T) {
	var a FlexibleAmount
	if err := json.Unmarshal([]byte(`1.5`), &a); err == nil {
		t.Fatalf("expected error for fractional value")
	}
}

func TestFlexibleAmount_MarshalAlwaysString(t *testing.T) {
	a := FlexibleAmount("12345")
	out, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != `"12345"` {
		t.Fatalf("got %s, want %s", out, `"12345"`)
	}
}

func TestFlexibleAmount_EmptyDefaultsToZero(t *testing.T) {
	var a FlexibleAmount
	if a.String() != "0" {
		t.Fatalf("zero value: got %q, want %q", a, "0")
	}
}

func TestSendMultipleRequest_FullRoundTrip(t *testing.T) {
	// Mirrors a payload the upstream txgen-basic.sh shell driver emits.
	raw := []byte(`{
		"value": 1,
		"numOfTxs": 250,
		"gasPrice": 1000000000,
		"gasLimit": 50000,
		"destination": "mixed",
		"recallNonce": false,
		"scenario": "basic"
	}`)
	var req SendMultipleRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("decode upstream payload: %v", err)
	}
	if req.Value.String() != "1" {
		t.Fatalf("value: got %q, want %q", req.Value, "1")
	}
	if req.NumOfTxs != 250 {
		t.Fatalf("numOfTxs: got %d, want 250", req.NumOfTxs)
	}
	if req.GasPrice != 1_000_000_000 {
		t.Fatalf("gasPrice: got %d", req.GasPrice)
	}
	if req.Scenario != "basic" {
		t.Fatalf("scenario: got %q", req.Scenario)
	}
}
