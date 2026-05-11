package scenarios

import (
	"encoding/hex"
	"math/big"
	"strings"
)

// EncodeData builds a MultiversX-style transaction data field by joining
// the supplied parts with '@' separators. Each part is supplied already
// hex-encoded. The first part is the function or built-in name (left as
//-is for readability per upstream convention; senders may also pre-hex-
// encode it if they prefer, the chain accepts both forms).
//
// Example: EncodeData("ESDTTransfer", HexBytes(token), HexBigInt(amount))
//   ->     "ESDTTransfer@4d59544f4b454e2d616263313233@03e8"
func EncodeData(funcName string, hexArgs ...string) string {
	parts := make([]string, 0, 1+len(hexArgs))
	parts = append(parts, funcName)
	parts = append(parts, hexArgs...)
	return strings.Join(parts, "@")
}

// HexBytes returns the lowercase hex encoding of b.
func HexBytes(b []byte) string {
	return hex.EncodeToString(b)
}

// HexString hex-encodes a string as its UTF-8 bytes. Useful for token
// names, function arguments expressed as bytes.
func HexString(s string) string {
	return hex.EncodeToString([]byte(s))
}

// HexBigInt returns the minimal big-endian hex encoding of a non-negative
// big.Int. Zero is encoded as "" — that is the MultiversX convention for
// the smallest possible nonzero-aware representation.
//
// For amounts that must be transmitted as exactly N bytes (e.g. fixed-width
// counters), use HexBigIntPadded.
func HexBigInt(v *big.Int) string {
	if v == nil || v.Sign() == 0 {
		return ""
	}
	return hex.EncodeToString(v.Bytes())
}

// HexBigIntPadded encodes v left-padded to width bytes. Width must be at
// least len(v.Bytes()); otherwise the high bytes would be silently lost.
func HexBigIntPadded(v *big.Int, width int) string {
	if v == nil {
		return strings.Repeat("00", width)
	}
	b := v.Bytes()
	if len(b) >= width {
		return hex.EncodeToString(b)
	}
	padded := make([]byte, width)
	copy(padded[width-len(b):], b)
	return hex.EncodeToString(padded)
}

// HexUint64 encodes a uint64 minimally — leading zero bytes are stripped.
func HexUint64(v uint64) string {
	return HexBigInt(new(big.Int).SetUint64(v))
}

// MustBigIntFromString parses a decimal big integer, panicking on error.
// Callers should validate input before invocation.
func MustBigIntFromString(s string) *big.Int {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("invalid big integer: " + s)
	}
	return v
}
