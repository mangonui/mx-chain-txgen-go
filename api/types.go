package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// FlexibleAmount accepts either a JSON number or a JSON string and stores
// the result as a decimal string. The upstream txgen shell drivers emit
// `"value": 1` (a JSON number) while production wallets emit
// `"value": "1000000000000000000"` (a string for big-int safety). This
// type normalises both.
type FlexibleAmount string

// UnmarshalJSON satisfies encoding/json.Unmarshaler.
func (a *FlexibleAmount) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if len(s) == 0 {
		*a = "0"
		return nil
	}
	if s[0] == '"' {
		// quoted form — strip quotes and accept as-is
		var inner string
		if err := json.Unmarshal(b, &inner); err != nil {
			return fmt.Errorf("flexible amount string: %w", err)
		}
		*a = FlexibleAmount(inner)
		return nil
	}
	// numeric form — round trip via float to drop spurious ".0", then
	// require the result to be a non-negative integer.
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("flexible amount number: %w", err)
	}
	if f < 0 || f != float64(int64(f)) {
		return fmt.Errorf("flexible amount must be a non-negative integer, got %s", s)
	}
	*a = FlexibleAmount(strconv.FormatInt(int64(f), 10))
	return nil
}

// MarshalJSON emits the canonical string form, regardless of input shape.
func (a FlexibleAmount) MarshalJSON() ([]byte, error) {
	if a == "" {
		return []byte(`"0"`), nil
	}
	return []byte(`"` + string(a) + `"`), nil
}

// String returns the underlying decimal string.
func (a FlexibleAmount) String() string {
	if a == "" {
		return "0"
	}
	return string(a)
}

// SendMultipleRequest is the JSON body of POST /transaction/send-multiple.
// Field names and semantics match the upstream txgen's contract so the
// existing txgen-*.sh shell drivers in mx-chain-go work unchanged.
//
// Version and Options are extensions for testing alternative tx shapes
// on Supernova (e.g. Version=2 with Options=1 to request hash-on-sign,
// or Options bits used by guarded / relayed transactions). When omitted
// they default to Version=1, Options=0 — the historically safe choice
// for move-balance and standard contract calls.
type SendMultipleRequest struct {
	Value       FlexibleAmount `json:"value"`
	NumOfTxs    int            `json:"numOfTxs"`
	GasPrice    uint64         `json:"gasPrice"`
	GasLimit    uint64         `json:"gasLimit"`
	Destination string         `json:"destination"`
	RecallNonce bool           `json:"recallNonce"`
	Scenario    string         `json:"scenario"`
	Data        string         `json:"data,omitempty"`
	SCAddress   string         `json:"scAddress,omitempty"`
	Version     uint32         `json:"version,omitempty"`
	Options     uint32         `json:"options,omitempty"`
}

// SendMultipleResponse mirrors the upstream proxy's response shape so the
// shell drivers' "data" / "code" presence checks succeed.
type SendMultipleResponse struct {
	Data  SendMultipleResponseData `json:"data"`
	Error string                   `json:"error"`
	Code  string                   `json:"code"`
}

// SendMultipleResponseData carries the actual outcome.
type SendMultipleResponseData struct {
	NumOfSentTxs int               `json:"numOfSentTxs"`
	TxsHashes    map[int]string    `json:"txsHashes"`
	Scenario     string            `json:"scenario"`
	SubCommand   string            `json:"subCommand,omitempty"`
	Extra        map[string]any    `json:"extra,omitempty"`
}
